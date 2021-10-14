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
//go:generate mockgen -destination ./mock/mock_source_client.go -package mock d7y.io/dragonfly/v2/pkg/source ResourceClient

package source

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"github.com/pkg/errors"
)

var _ ResourceClient = (*clientManager)(nil)
var _ ClientManager = (*clientManager)(nil)

// ResourceClient define apis that interact with the source.
type ResourceClient interface {

	// GetContentLength get length of resource content
	// return -l if request fail
	// return task.IllegalSourceFileLen if response status is not StatusOK and StatusPartialContent
	GetContentLength(ctx context.Context, request *Request) (int64, error)

	// IsSupportRange checks if resource supports breakpoint continuation
	IsSupportRange(ctx context.Context, request *Request) (bool, error)

	// IsExpired checks if a resource received or stored is the same.
	IsExpired(ctx context.Context, request *Request) (bool, error)

	// Download downloads from source
	Download(ctx context.Context, request *Request) (io.ReadCloser, error)

	// DownloadWithResponseHeader download from source with responseHeader
	DownloadWithResponseHeader(ctx context.Context, request *Request) (*Response, error)

	// GetLastModifiedMillis gets last modified timestamp milliseconds of resource
	GetLastModifiedMillis(ctx context.Context, request *Request) (int64, error)
}

const (
	LastModified = "Last-Modified"
	ETag         = "ETag"
)

type ClientManager interface {
	ResourceClient
	Register(schema string, resourceClient ResourceClient)
	UnRegister(schema string)
}

type clientManager struct {
	sync.RWMutex
	clients map[string]ResourceClient
}

var _defaultManager = NewManager()

func (m *clientManager) GetContentLength(ctx context.Context, request *Request) (int64, error) {
	sourceClient, err := m.getSourceClient(request.URL.Scheme)
	if err != nil {
		return -1, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	return sourceClient.GetContentLength(ctx, request)
}

func (m *clientManager) IsSupportRange(ctx context.Context, request *Request) (bool, error) {
	sourceClient, err := m.getSourceClient(request.URL.Scheme)
	if err != nil {
		return false, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	return sourceClient.IsSupportRange(ctx, request)
}

func (m *clientManager) IsExpired(ctx context.Context, request *Request) (bool, error) {
	sourceClient, err := m.getSourceClient(request.URL.Scheme)
	if err != nil {
		return false, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	return sourceClient.IsExpired(ctx, request)
}

func (m *clientManager) Download(ctx context.Context, request *Request) (io.ReadCloser, error) {
	sourceClient, err := m.getSourceClient(request.URL.Scheme)
	if err != nil {
		return nil, err
	}
	return sourceClient.Download(ctx, request)
}

func (m *clientManager) DownloadWithResponseHeader(ctx context.Context, request *Request) (*Response, error) {
	sourceClient, err := m.getSourceClient(request.URL.Scheme)
	if err != nil {
		return nil, err
	}
	return sourceClient.DownloadWithResponseHeader(ctx, request)
}

func (m *clientManager) GetLastModifiedMillis(ctx context.Context, request *Request) (int64, error) {
	sourceClient, err := m.getSourceClient(request.URL.Scheme)
	if err != nil {
		return -1, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
	}
	return sourceClient.GetLastModifiedMillis(ctx, request)
}

func NewManager() ClientManager {
	return &clientManager{
		clients: make(map[string]ResourceClient),
	}
}

func (m *clientManager) Register(schema string, resourceClient ResourceClient) {
	if client, ok := m.clients[strings.ToLower(schema)]; ok {
		logger.Infof("replace client %#v with %#v for schema %s", client, resourceClient, schema)
	}
	m.clients[strings.ToLower(schema)] = resourceClient
}

func Register(schema string, resourceClient ResourceClient) {
	_defaultManager.Register(schema, resourceClient)
}

func (m *clientManager) UnRegister(schema string) {
	if client, ok := m.clients[strings.ToLower(schema)]; ok {
		logger.Infof("remove client %#v for schema %s", client, schema)
	}
	delete(m.clients, strings.ToLower(schema))
}

func UnRegister(schema string) {
	_defaultManager.UnRegister(schema)
}

func GetContentLength(ctx context.Context, request *Request) (int64, error) {
	return _defaultManager.GetContentLength(ctx, request)
}

func IsSupportRange(ctx context.Context, request *Request) (bool, error) {
	return _defaultManager.IsSupportRange(ctx, request)
}

func IsExpired(ctx context.Context, request *Request) (bool, error) {
	return _defaultManager.IsExpired(ctx, request)
}

func Download(ctx context.Context, request *Request) (io.ReadCloser, error) {
	return _defaultManager.Download(ctx, request)
}

func DownloadWithResponseHeader(ctx context.Context, request *Request) (*Response, error) {
	return _defaultManager.DownloadWithResponseHeader(ctx, request)
}

func GetLastModifiedMillis(ctx context.Context, request *Request) (int64, error) {
	return _defaultManager.GetLastModifiedMillis(ctx, request)
}

// getSourceClient get a source client from source manager with specified schema.
func (m *clientManager) getSourceClient(schema string) (ResourceClient, error) {
	logger.Debugf("current clients: %#v", m.clients)
	m.RLock()
	client, ok := m.clients[strings.ToLower(schema)]
	m.RUnlock()
	if !ok || client == nil {
		return nil, errors.Errorf("can not find client supporting schema %s, clients:%v", schema, m.clients)
	}
	return client, nil
}

func (m *clientManager) loadSourcePlugin(schema string) (ResourceClient, error) {
	m.Lock()
	defer m.Unlock()
	// double check
	client, ok := m.clients[schema]
	if ok {
		return client, nil
	}

	client, err := LoadPlugin(schema)
	if err != nil {
		return nil, err
	}
	m.clients[schema] = client
	return client, nil
}
