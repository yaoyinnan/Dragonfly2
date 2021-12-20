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

package proxyprotocol

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-http-utils/headers"

	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/timeutils"
)

const (
	proxyClient = "proxy"
)

var _ source.ResourceClient = (*proxySourceClient)(nil)

func init() {
	if err := source.Register(proxyClient, newProxySourceClient(), Adapter); err != nil {
		panic(err)
	}
}

func Adapter(request *source.Request) *source.Request {
	clonedRequest := request.Clone(request.Context())
	return clonedRequest
}

// proxySourceClient is an implementation of the interface of source.ResourceClient.
type proxySourceClient struct {
	client *http.Client
}

func newProxySourceClient(opts ...ProxySourceClientOption) *proxySourceClient {
	client := &proxySourceClient{
		client: http.DefaultClient,
	}
	for i := range opts {
		opts[i](client)
	}
	return client
}

type ProxySourceClientOption func(p *proxySourceClient)

func WithHTTPClient(client *http.Client) ProxySourceClientOption {
	return func(sourceClient *proxySourceClient) {
		sourceClient.client = client
	}
}

const (
	GetContentLength = "contentLength"
	IsSupportRange   = "supportRange"
	IsExpired        = "expired"
	Download         = "download"
	GetLastModified  = "lastModified"
)

func (client *proxySourceClient) GetContentLength(request *source.Request) (int64, error) {
	resp, err := client.doRequest(request, GetContentLength)
	if err != nil {
		return source.UnknownSourceFileLen, err
	}
	defer resp.Body.Close()
	err = source.CheckResponseCode(resp.StatusCode, []int{http.StatusOK, http.StatusPartialContent})
	if err != nil {
		return source.UnknownSourceFileLen, err
	}
	return resp.ContentLength, nil
}

func (client *proxySourceClient) IsSupportRange(request *source.Request) (bool, error) {
	resp, err := client.doRequest(request, IsSupportRange)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusPartialContent, nil
}

func (client *proxySourceClient) IsExpired(request *source.Request, info *source.ExpireInfo) (bool, error) {
	resp, err := client.doRequest(request, IsExpired)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return !(resp.StatusCode == http.StatusNotModified || (resp.Header.Get(headers.ETag) == info.ETag || resp.Header.Get(headers.LastModified) == info.
		LastModified)), nil
}

func (client *proxySourceClient) Download(request *source.Request) (*source.Response, error) {
	resp, err := client.doRequest(request, Download)
	if err != nil {
		return nil, err
	}
	err = source.CheckResponseCode(resp.StatusCode, []int{http.StatusOK, http.StatusPartialContent})
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	response := source.NewResponse(
		resp.Body,
		source.WithExpireInfo(
			source.ExpireInfo{
				LastModified: resp.Header.Get(source.LastModified),
				ETag:         resp.Header.Get(source.ETag),
			},
		))
	return response, nil
}

func (client *proxySourceClient) GetLastModified(request *source.Request) (int64, error) {
	resp, err := client.doRequest(request, GetLastModified)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	err = source.CheckResponseCode(resp.StatusCode, []int{http.StatusOK, http.StatusPartialContent})
	if err != nil {
		return -1, err
	}
	return timeutils.UnixMillis(resp.Header.Get(source.LastModified)), nil
}

func (client *proxySourceClient) doRequest(request *source.Request, action string) (*http.Response, error) {
	proxyURL := fmt.Sprintf("%s?action=%s", strings.Replace(request.URL.String(), "proxy", "http", 1), action)
	req, err := http.NewRequestWithContext(request.Context(), http.MethodGet, proxyURL, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range request.Header {
		for i := range values {
			req.Header.Add(key, values[i])
		}
	}
	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
