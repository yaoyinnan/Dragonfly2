/*
 *     Copyright 2020 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//go:generate mockgen -destination peer_manager_mock.go -source peer_manager.go -package resource

package resource

import (
	"context"
	"sync"
	"time"

	pkggc "d7y.io/dragonfly/v2/pkg/gc"
	"d7y.io/dragonfly/v2/scheduler/config"
)

const (
	// GC peer id.
	GCPeerID = "peer"
)

type PeerManager interface {
	// Load returns peer for a key.
	Load(string) (*Peer, bool)

	// Store sets peer.
	Store(*Peer)

	// LoadOrStore returns peer the key if present.
	// Otherwise, it stores and returns the given peer.
	// The loaded result is true if the peer was loaded, false if stored.
	LoadOrStore(*Peer) (*Peer, bool)

	// Delete deletes peer for a key.
	Delete(string)

	// Try to reclaim peer.
	RunGC() error
}

type peerManager struct {
	// Peer sync map.
	*sync.Map

	// ttl is time to live of peer.
	ttl time.Duration

	// pieceDownloadTimeout is timeout of downloading piece.
	pieceDownloadTimeout time.Duration

	// mu is peer mutex.
	mu *sync.Mutex
}

// New peer manager interface.
func newPeerManager(cfg *config.GCConfig, gc pkggc.GC) (PeerManager, error) {
	p := &peerManager{
		Map:                  &sync.Map{},
		ttl:                  cfg.PeerTTL,
		pieceDownloadTimeout: cfg.PieceDownloadTimeout,
		mu:                   &sync.Mutex{},
	}

	if err := gc.Add(pkggc.Task{
		ID:       GCPeerID,
		Interval: cfg.PeerGCInterval,
		Timeout:  cfg.PeerGCInterval,
		Runner:   p,
	}); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *peerManager) Load(key string) (*Peer, bool) {
	rawPeer, ok := p.Map.Load(key)
	if !ok {
		return nil, false
	}

	return rawPeer.(*Peer), ok
}

func (p *peerManager) Store(peer *Peer) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Map.Store(peer.ID, peer)
	peer.Task.StorePeer(peer)
	peer.Host.StorePeer(peer)
}

func (p *peerManager) LoadOrStore(peer *Peer) (*Peer, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	rawPeer, loaded := p.Map.LoadOrStore(peer.ID, peer)
	if !loaded {
		peer.Host.StorePeer(peer)
		peer.Task.StorePeer(peer)
	}

	return rawPeer.(*Peer), loaded
}

func (p *peerManager) Delete(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if peer, ok := p.Load(key); ok {
		p.Map.Delete(key)
		peer.Task.DeletePeer(key)
		peer.Host.DeletePeer(key)
	}
}

func (p *peerManager) RunGC() error {
	p.Map.Range(func(_, value any) bool {
		peer, ok := value.(*Peer)
		if !ok {
			peer.Log.Warn("invalid peer")
			return true
		}

		// If the peer state is PeerStateLeave,
		// peer will be reclaimed.
		if peer.FSM.Is(PeerStateLeave) {
			p.Delete(peer.ID)
			peer.Log.Info("peer has been reclaimed")
			return true
		}

		// If the peer's elapsed of downloading piece exceeds the pieceDownloadTimeout,
		// then sets the peer state to PeerStateLeave and then delete peer.
		if peer.FSM.Is(PeerStateRunning) || peer.FSM.Is(PeerStateBackToSource) {
			elapsed := time.Since(peer.PieceUpdatedAt.Load())
			if elapsed > p.pieceDownloadTimeout {
				peer.Log.Info("peer elapsed exceeds the timeout of downloading piece, causing the peer to leave")
				if err := peer.FSM.Event(context.Background(), PeerEventLeave); err != nil {
					peer.Log.Errorf("peer fsm event failed: %s", err.Error())
					return true
				}

				return true
			}
		}

		// If the peer's elapsed exceeds the ttl,
		// then set the peer state to PeerStateLeave and then delete peer.
		elapsed := time.Since(peer.UpdatedAt.Load())
		if elapsed > p.ttl {
			peer.Log.Info("peer elapsed exceeds the ttl, causing the peer to leave")
			if err := peer.FSM.Event(context.Background(), PeerEventLeave); err != nil {
				peer.Log.Errorf("peer fsm event failed: %s", err.Error())
				return true
			}

			return true
		}

		// If the peer's state is PeerStateFailed,
		// then set the peer state to PeerStateLeave and then delete peer.
		if peer.FSM.Is(PeerStateFailed) {
			peer.Log.Info("peer state is PeerStateFailed, causing the peer to leave")
			if err := peer.FSM.Event(context.Background(), PeerEventLeave); err != nil {
				peer.Log.Errorf("peer fsm event failed: %s", err.Error())
				return true
			}
		}

		// If no peer exists in the dag of the task,
		// delete the peer.
		degree, err := peer.Task.PeerDegree(peer.ID)
		if err != nil {
			p.Delete(peer.ID)
			peer.Log.Info("peer has been reclaimed")
			return true
		}

		// If the task dag size exceeds the limit,
		// then set the peer state to PeerStateLeave which state is
		// PeerStateSucceeded, and degree is zero.
		if peer.Task.PeerCount() > PeerCountLimitForTask &&
			peer.FSM.Is(PeerStateSucceeded) && degree == 0 {
			peer.Log.Info("task dag size exceeds the limit, causing the peer to leave")
			if err := peer.FSM.Event(context.Background(), PeerEventLeave); err != nil {
				peer.Log.Errorf("peer fsm event failed: %s", err.Error())
				return true
			}

			return true
		}

		return true
	})

	return nil
}
