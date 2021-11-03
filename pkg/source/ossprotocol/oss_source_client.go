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

package ossprotocol

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	cdnerrors "d7y.io/dragonfly/v2/cdn/errors"
	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/go-http-utils/headers"
	"github.com/pkg/errors"
)

const ossClient = "oss"

const (
	endpoint        = "endpoint"
	accessKeyID     = "accessKeyID"
	accessKeySecret = "accessKeySecret"
)

var _ source.ResourceClient = (*ossSourceClient)(nil)

func init() {
	sourceClient := NewOSSSourceClient()
	source.Register(ossClient, sourceClient, adaptor)
}

func adaptor(request *source.Request) *source.Request {
	clonedRequest := request.Clone(request.Context())
	if request.Header.Get(source.Range) != "" {
		clonedRequest.Header.Set(headers.Range, fmt.Sprintf("bytes=%s", request.Header.Get(source.Range)))
		clonedRequest.Header.Del(source.Range)
	}
	if request.Header.Get(source.LastModified) != "" {
		clonedRequest.Header.Set(headers.LastModified, request.Header.Get(source.LastModified))
		clonedRequest.Header.Del(source.LastModified)
	}
	if request.Header.Get(source.ETag) != "" {
		clonedRequest.Header.Set(headers.ETag, request.Header.Get(source.ETag))
		clonedRequest.Header.Del(source.ETag)
	}
	return clonedRequest
}

func NewOSSSourceClient(opts ...OssSourceClientOption) source.ResourceClient {
	sourceClient := &ossSourceClient{
		clientMap: sync.Map{},
		accessMap: sync.Map{},
	}
	for i := range opts {
		opts[i](sourceClient)
	}
	return sourceClient
}

type OssSourceClientOption func(p *ossSourceClient)

// ossSourceClient is an implementation of the interface of SourceClient.
type ossSourceClient struct {
	// endpoint_accessKeyID_accessKeySecret -> ossClient
	clientMap sync.Map
	accessMap sync.Map
}

func (osc *ossSourceClient) TransformToConcreteHeader(header source.Header) source.Header {
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

func (osc *ossSourceClient) Download(request *source.Request) (io.ReadCloser, error) {
	panic("implement me")
}

func (osc *ossSourceClient) GetLastModifiedMillis(request *source.Request) (int64, error) {
	panic("implement me")
}

func (osc *ossSourceClient) GetContentLength(request *source.Request) (int64, error) {
	resHeader, err := osc.getMeta(request)
	if err != nil {
		return types.UnKnownSourceFileLen, err
	}

	contentLen, err := strconv.ParseInt(resHeader.Get(oss.HTTPHeaderContentLength), 10, 64)
	if err != nil {
		return -1, err
	}

	return contentLen, nil
}

func (osc *ossSourceClient) IsSupportRange(request *source.Request) (bool, error) {
	_, err := osc.getMeta(request)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (osc *ossSourceClient) IsExpired(request *source.Request) (bool, error) {
	resHeader, err := osc.getMeta(request)
	if err != nil {
		return false, err
	}
	return resHeader.Get(oss.HTTPHeaderLastModified) == request.Header.Get(oss.HTTPHeaderLastModified) && resHeader.Get(oss.HTTPHeaderEtag) == request.Header.Get(oss.
		HTTPHeaderEtag), nil
}

func (osc *ossSourceClient) DownloadWithExpireInfo(request *source.Request) (io.ReadCloser, *source.ExpireInfo, error) {
	ossObject, err := parseOssObject(request.URL)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "parse oss object from url: %s", request.URL.String())
	}
	client, err := osc.getClient(request.Header)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get client")
	}
	bucket, err := client.Bucket(ossObject.bucket)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get bucket: %s", ossObject.bucket)
	}
	res, err := bucket.GetObject(ossObject.object, getOptions(request.Header)...)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get oss Object: %s", ossObject.object)
	}
	resp := res.(*oss.Response)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		return resp.Body, &source.ExpireInfo{
			LastModified: resp.Headers.Get(headers.LastModified),
			ETag:         resp.Headers.Get(headers.ETag),
		}, nil
	}
	resp.Body.Close()
	return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

func (osc *ossSourceClient) Transform(header source.Header) source.Header {
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

func (osc *ossSourceClient) getClient(header source.Header) (*oss.Client, error) {
	endpoint := header.Get(endpoint)
	if stringutils.IsBlank(endpoint) {
		return nil, errors.Wrapf(cdnerrors.ErrInvalidValue, "endpoint is empty")
	}
	accessKeyID := header.Get(accessKeyID)
	if stringutils.IsBlank(accessKeyID) {
		return nil, errors.Wrapf(cdnerrors.ErrInvalidValue, "accessKeyID is empty")
	}
	accessKeySecret := header.Get(accessKeySecret)
	if stringutils.IsBlank(accessKeySecret) {
		return nil, errors.Wrapf(cdnerrors.ErrInvalidValue, "accessKeySecret is empty")
	}
	clientKey := genClientKey(endpoint, accessKeyID, accessKeySecret)
	if client, ok := osc.clientMap.Load(clientKey); ok {
		return client.(*oss.Client), nil
	}
	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}
	osc.clientMap.Store(clientKey, client)
	return client, nil
}

func genClientKey(endpoint, accessKeyID, accessKeySecret string) string {
	return fmt.Sprintf("%s_%s_%s", endpoint, accessKeyID, accessKeySecret)
}

func (osc *ossSourceClient) getMeta(request *source.Request) (http.Header, error) {
	client, err := osc.getClient(request.Header)
	if err != nil {
		return nil, errors.Wrapf(err, "get oss client")
	}
	ossObject, err := parseOssObject(request.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "parse oss object")
	}

	bucket, err := client.Bucket(ossObject.bucket)
	if err != nil {
		return nil, errors.Wrapf(err, "get bucket: %s", ossObject.bucket)
	}
	isExist, err := bucket.IsObjectExist(ossObject.object)
	if err != nil {
		return nil, errors.Wrapf(err, "prob object: %s if exist", ossObject.object)
	}
	if !isExist {
		return nil, fmt.Errorf("oss object: %s does not exist", ossObject.object)
	}
	return bucket.GetObjectMeta(ossObject.object, getOptions(request.Header)...)
}

func getOptions(header source.Header) []oss.Option {
	opts := make([]oss.Option, 0, len(header))
	for key, value := range header {
		if key == endpoint || key == accessKeyID || key == accessKeySecret {
			continue
		}
		opts = append(opts, oss.SetHeader(key, value))
	}
	return opts
}

type ossObject struct {
	endpoint string
	bucket   string
	object   string
}

func parseOssObject(url *url.URL) (*ossObject, error) {
	if url.Scheme != "oss" {
		return nil, fmt.Errorf("rawUrl: %s is not oss url", url.String())
	}
	return &ossObject{
		endpoint: url.Path[0:2],
		bucket:   url.Host,
		object:   url.Path[1:],
	}, nil
}
