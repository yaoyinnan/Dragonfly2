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

package httpprotocol

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/timeutils"
	"github.com/go-http-utils/headers"
	"github.com/pkg/errors"
)

const (
	HTTPClient  = "http"
	HTTPSClient = "https"

	ProxyEnv = "D7Y_SOURCE_PROXY"
)

var _defaultHTTPClient *http.Client
var _ source.ResourceClient = (*httpSourceClient)(nil)

func init() {
	// TODO support customize source client
	var (
		proxy *url.URL
		err   error
	)
	if proxyEnv := os.Getenv(ProxyEnv); len(proxyEnv) > 0 {
		proxy, err = url.Parse(proxyEnv)
		if err != nil {
			fmt.Printf("Back source proxy parse error: %s\n", err)
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = true
	transport.DialContext = (&net.Dialer{
		Timeout:   3 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	if proxy != nil {
		transport.Proxy = http.ProxyURL(proxy)
	}

	_defaultHTTPClient = &http.Client{
		Transport: transport,
	}
	sc := NewHTTPSourceClient()
	source.Register(HTTPClient, sc)
	source.Register(HTTPSClient, sc)
}

// httpSourceClient is an implementation of the interface of source.ResourceClient.
type httpSourceClient struct {
	httpClient *http.Client
}

// NewHTTPSourceClient returns a new HTTPSourceClientOption.
func NewHTTPSourceClient(opts ...HTTPSourceClientOption) source.ResourceClient {
	client := &httpSourceClient{
		httpClient: _defaultHTTPClient,
	}
	for i := range opts {
		opts[i](client)
	}
	return client
}

type HTTPSourceClientOption func(p *httpSourceClient)

func WithHTTPClient(client *http.Client) HTTPSourceClientOption {
	return func(sourceClient *httpSourceClient) {
		sourceClient.httpClient = client
	}
}

func (client *httpSourceClient) GetContentLength(request *source.Request) (int64, error) {
	resp, err := client.doRequest(http.MethodGet, request)
	if err != nil {
		return types.UnKnownSourceFileLen, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		//similar to proposing another error type to indicate that this  error can interact with the URL, but the status code does not meet expectations
		return types.UnKnownSourceFileLen, &source.ErrUnExpectedResponse{StatusCode: resp.StatusCode, Status: resp.Status}
	}
	return resp.ContentLength, nil
}

func (client *httpSourceClient) IsSupportRange(request *source.Request) (bool, error) {
	request.Header.Add(headers.Range, "bytes=0-0")
	resp, err := client.doRequest(http.MethodGet, request)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusPartialContent, nil
}

func (client *httpSourceClient) IsExpired(request *source.Request) (bool, error) {
	resp, err := client.doRequest(http.MethodGet, request)
	// send request
	if err != nil {
		// If it fails to get the result, it is considered that the source has not expired, to prevent the source from exploding
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode != http.StatusNotModified, nil
}

func (client *httpSourceClient) Download(request *source.Request) (io.ReadCloser, error) {
	resp, err := client.doRequest(http.MethodGet, request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		return resp.Body, nil
	}
	defer resp.Body.Close()
	return nil, &source.ErrUnExpectedResponse{StatusCode: resp.StatusCode, Status: resp.Status}
}

func (client *httpSourceClient) DownloadWithResponseHeader(request *source.Request) (*source.Response, error) {
	resp, err := client.doRequest(http.MethodGet, request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		response := &source.Response{
			Status:        resp.Status,
			StatusCode:    resp.StatusCode,
			Header:        transformToSourceHeader(resp.Header),
			Body:          resp.Body,
			ContentLength: resp.ContentLength,
		}
		return response, nil
	}
	defer resp.Body.Close()
	return nil, &source.ErrUnExpectedResponse{StatusCode: resp.StatusCode, Status: resp.Status}
}

func (client *httpSourceClient) GetLastModifiedMillis(request *source.Request) (int64, error) {
	req, err := http.NewRequestWithContext(request.Context(), http.MethodGet, request.URL.String(), nil)
	if err != nil {
		return -1, err
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		return timeutils.UnixMillis(resp.Header.Get(headers.LastModified)), nil
	}
	return -1, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

func (client *httpSourceClient) TransformToConcreteHeader(header source.Header) source.Header {
	clonedHeader := header.Clone()
	if clonedHeader.Get(source.Range) != "" {
		clonedHeader.Set(headers.Range, fmt.Sprintf("bytes=%s", clonedHeader.Get(source.Range)))
	}
	if clonedHeader.Get(source.LastModified) != "" {
		clonedHeader.Set(headers.LastModified, clonedHeader.Get(source.LastModified))
	}
	if clonedHeader.Get(source.ETag) != "" {
		clonedHeader.Set(headers.ETag, clonedHeader.Get(source.ETag))
	}
	return clonedHeader
}

func transformToSourceHeader(httpHeader http.Header) source.Header {
	sourceHeader := source.Header{}
	for key, values := range httpHeader {
		for i := range values {
			sourceHeader.Add(key, values[i])
		}
	}
	if sourceHeader.Get(headers.Range) != "" {
		sourceHeader.Set(source.Range, strings.Split(sourceHeader.Get(headers.Range), "=")[1])
	}
	if sourceHeader.Get(headers.LastModified) != "" {
		sourceHeader.Set(source.LastModified, sourceHeader.Get(headers.LastModified))
	}
	if sourceHeader.Get(headers.ETag) != "" {
		sourceHeader.Set(source.ETag, sourceHeader.Get(headers.ETag))
	}
	return sourceHeader
}

func (client *httpSourceClient) doRequest(method string, request *source.Request) (*http.Response, error) {
	req, err := http.NewRequestWithContext(request.Context(), method, request.URL.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request")
	}
	for key, values := range request.Header {
		for i := range values {
			req.Header.Add(key, values[i])
		}
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "request source resource")
	}
	return resp, nil
}
