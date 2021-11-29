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

package progress

type Observer interface {
	Notify(seedPiece *SeedPiece)
	Receiver() <-chan *SeedPiece
}

type observer struct {
	pieces    []*SeedPiece
	pieceChan chan *SeedPiece
}

func (o *observer) Notify(seedPiece *SeedPiece) {
	o.pieceChan <- seedPiece
}

func (o *observer) Receiver() <-chan *SeedPiece {
	return o.pieceChan
}

type Subject interface {
	AddObserver(observer Observer)

	RemoveObserver(observer observer)

	NotifyObservers(seedPiece *SeedPiece)
}

type subject struct {
	observers []Observer
}

func (s *subject) AddObserver(observer Observer) {
	s.observers = append(s.observers, observer)
}

func (s *subject) RemoveObserver(observer observer) {
	s.observers = append(s.observers[:1], s.observers[2:]...)
}

func (s *subject) NotifyObservers(seedPiece *SeedPiece) {

}
