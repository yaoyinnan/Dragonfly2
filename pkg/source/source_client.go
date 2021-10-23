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
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"github.com/pkg/errors"
)

// ErrUnExpectedResponse represents the response is not expected
type ErrUnExpectedResponse struct {
	StatusCode int
	Status     string
}

func (e *ErrUnExpectedResponse) Error() string {
	return fmt.Sprintf("Status: %s, StatusCode: %d", e.Status, e.StatusCode)
}

func IsUnExpectedResponse(err error) bool {
	err = errors.Cause(err)
	_, ok := err.(*ErrUnExpectedResponse)
	return ok
}

var ErrNoClientFound = errors.New("no source client found")

// ResourceClient define apis that interact with the source.
type ResourceClient interface {

	// GetContentLength get length of resource content
	// return types.UnKnownSourceFileLen if response status is not StatusOK and StatusPartialContent
	GetContentLength(request *Request) (int64, error)

	// IsSupportRange checks if resource supports breakpoint continuation
	IsSupportRange(request *Request) (bool, error)

	// IsExpired checks if a resource received or stored is the same.
	// If it fails to get the result, it is considered that the source has not expired, return false and non-nil err to prevent the source from exploding
	IsExpired(request *Request) (bool, error)

	// Download downloads from source
	Download(request *Request) (io.ReadCloser, error)

	// DownloadWithResponseHeader download from source with responseHeader
	DownloadWithResponseHeader(request *Request) (*Response, error)

	// GetLastModifiedMillis gets last modified timestamp milliseconds of resource
	GetLastModifiedMillis(request *Request) (int64, error)

	// TransformToConcreteHeader source header tag to concrete source header tag
	TransformToConcreteHeader(header Header) Header
}

type ClientManager interface {
	// Register a source client with scheme
	Register(scheme string, resourceClient ResourceClient)

	// UnRegister a source client from manager
	UnRegister(scheme string)

	// GetClient a source client by scheme
	GetClient(scheme string) (ResourceClient, bool)
}

// clientManager implements the interface ClientManager
type clientManager struct {
	mu      sync.RWMutex
	clients map[string]ResourceClient
}

var _ ClientManager = (*clientManager)(nil)

var _defaultManager = NewManager()

func NewManager() ClientManager {
	return &clientManager{
		clients: make(map[string]ResourceClient),
	}
}

func (m *clientManager) Register(scheme string, resourceClient ResourceClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.clients[strings.ToLower(scheme)]; ok {
		logger.Infof("replace client %#v with %#v for scheme %s", client, resourceClient, scheme)
	}
	m.clients[strings.ToLower(scheme)] = resourceClient
}

func (m *clientManager) UnRegister(scheme string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, ok := m.clients[strings.ToLower(scheme)]; ok {
		logger.Infof("remove client %#v for scheme %s", client, scheme)
	}
	delete(m.clients, strings.ToLower(scheme))
}

func (m *clientManager) GetClient(scheme string) (ResourceClient, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	client, ok := m.clients[strings.ToLower(scheme)]
	return client, ok
}

func Register(scheme string, resourceClient ResourceClient) {
	_defaultManager.Register(scheme, resourceClient)
}

func UnRegister(scheme string) {
	_defaultManager.UnRegister(scheme)
}

func GetContentLength(request *Request) (int64, error) {
	client, ok := _defaultManager.GetClient(request.URL.Scheme)
	if !ok {
		return types.UnKnownSourceFileLen, ErrNoClientFound
	}
	if _, ok := request.Context().Deadline(); !ok {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		request = request.WithContext(ctx)
		defer cancel()
	}
	request.Header = client.TransformToConcreteHeader(request.Header)
	return client.GetContentLength(request)
}

func IsSupportRange(request *Request) (bool, error) {
	client, ok := _defaultManager.GetClient(request.URL.Scheme)
	if !ok {
		return false, ErrNoClientFound
	}
	if _, ok := request.Context().Deadline(); !ok {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		request = request.WithContext(ctx)
		defer cancel()
	}
	request.Header = client.TransformToConcreteHeader(request.Header)
	if request.Header.get(Range) == "" {
		request.Header.Add(Range, "0-0")
	}
	request.Header = client.TransformToConcreteHeader(request.Header)
	return client.IsSupportRange(request)
}

func IsExpired(request *Request) (bool, error) {
	client, ok := _defaultManager.GetClient(request.URL.Scheme)
	if !ok {
		return false, ErrNoClientFound
	}
	if _, ok := request.Context().Deadline(); !ok {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		request = request.WithContext(ctx)
		defer cancel()
	}

	//lastModified := timeutils.UnixMillis(expireInfo[source.LastModified])
	//
	//eTag := expireInfo[headers.ETag]
	//if lastModified <= 0 && stringutils.IsBlank(eTag) {
	//	return true, nil
	//}
	//
	//if lastModified > 0 {
	//	copied[headers.IfModifiedSince] = expireInfo[headers.LastModified]
	//}
	//if !stringutils.IsBlank(eTag) {
	//	copied[headers.IfNoneMatch] = eTag
	//}
	return client.IsExpired(request)
}

func GetLastModifiedMillis(request *Request) (int64, error) {
	client, ok := _defaultManager.GetClient(request.URL.Scheme)
	if !ok {
		return -1, ErrNoClientFound
	}
	if _, ok := request.Context().Deadline(); !ok {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		request = request.WithContext(ctx)
		defer cancel()
	}
	return client.GetLastModifiedMillis(request)
}

func Download(request *Request) (io.ReadCloser, error) {
	client, ok := _defaultManager.GetClient(request.URL.Scheme)
	if !ok {
		return nil, ErrNoClientFound
	}
	request.Header = client.TransformToConcreteHeader(request.Header)
	return client.Download(request)
}

func DownloadWithResponseHeader(request *Request) (*Response, error) {
	client, ok := _defaultManager.GetClient(request.URL.Scheme)
	if !ok {
		return nil, ErrNoClientFound
	}
	request.Header = client.TransformToConcreteHeader(request.Header)
	return client.DownloadWithResponseHeader(request)
}

func (m *clientManager) loadSourcePlugin(scheme string) (ResourceClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// double check
	client, ok := m.clients[scheme]
	if ok {
		return client, nil
	}

	client, err := LoadPlugin(scheme)
	if err != nil {
		return nil, err
	}
	m.clients[scheme] = client
	return client, nil
}
