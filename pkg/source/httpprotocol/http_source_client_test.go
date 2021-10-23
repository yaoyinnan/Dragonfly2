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
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/rangeutils"
	"github.com/go-http-utils/headers"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/suite"
)

func TestHTTPSourceClientTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPSourceClientTestSuite))
}

type HTTPSourceClientTestSuite struct {
	suite.Suite
	source.ResourceClient
}

func (suite *HTTPSourceClientTestSuite) SetupSuite() {
	suite.ResourceClient = NewHTTPSourceClient()
	httpmock.ActivateNonDefault(_defaultHTTPClient)
}

func (suite *HTTPSourceClientTestSuite) TearDownSuite() {
	httpmock.DeactivateAndReset()
}

var (
	timeoutRawURL                   = "http://timeout.com"
	timeoutURL, _                   = url.Parse(timeoutRawURL)
	normalRawURL                    = "http://normal.com"
	normalURL, _                    = url.Parse(normalRawURL)
	normalRequest, _                = source.NewRequest(normalRawURL)
	errorRawURL                     = "http://error.com"
	errorURL, _                     = url.Parse(errorRawURL)
	errorRequest, _                 = source.NewRequest(errorRawURL)
	forbiddenRawURL                 = "http://forbidden.com"
	forbiddenURL, _                 = url.Parse(forbiddenRawURL)
	forbiddenRequest, _             = source.NewRequest(forbiddenRawURL)
	notfoundRawURL                  = "http://notfound.com"
	notfoundURL, _                  = url.Parse(notfoundRawURL)
	notfoundRequest, _              = source.NewRequest(notfoundRawURL)
	normalNotSupportRangeRawURL     = "http://notsuppertrange.com"
	normalNotSupportRangeURL, _     = url.Parse(normalNotSupportRangeRawURL)
	normalNotSupportRangeRequest, _ = source.NewRequest(normalNotSupportRangeRawURL)
)

var (
	testContent  = "l am test case"
	lastModified = "Sun, 06 Jun 2021 12:52:30 GMT"
	//etag         = "UMiJT4h7MCEAEgnqCLA2CdAaABnK" // todo etag business code can not obtain
	etag = ""
)

func (suite *HTTPSourceClientTestSuite) SetupTest() {
	httpmock.Reset()
	httpmock.RegisterResponder(http.MethodGet, timeoutRawURL, func(request *http.Request) (*http.Response, error) {
		// To simulate the timeout
		time.Sleep(5 * time.Second)
		return httpmock.NewStringResponse(http.StatusOK, "ok"), nil
	})

	httpmock.RegisterResponder(http.MethodGet, normalRawURL, func(request *http.Request) (*http.Response, error) {
		if rang := request.Header.Get(headers.Range); rang != "" {
			r, _ := rangeutils.ParseRange(rang[6:])
			res := &http.Response{
				StatusCode:    http.StatusPartialContent,
				ContentLength: int64(r.EndIndex) - int64(r.StartIndex) + int64(1),
				Body:          httpmock.NewRespBodyFromString(testContent[r.StartIndex:r.EndIndex]),
				Header: http.Header{
					headers.LastModified: []string{lastModified},
					headers.ETag:         []string{etag},
				},
			}
			return res, nil
		}
		if expire := request.Header.Get(headers.IfModifiedSince); expire != "" {
			res := &http.Response{
				StatusCode:    http.StatusNotModified,
				ContentLength: int64(len(testContent)),
				Body:          httpmock.NewRespBodyFromString(testContent),
				Header: http.Header{
					headers.LastModified: []string{lastModified},
					headers.ETag:         []string{etag},
				},
			}
			return res, nil
		}
		res := &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: 14,
			Body:          httpmock.NewRespBodyFromString(testContent),
			Header: http.Header{
				headers.LastModified: []string{lastModified},
				headers.ETag:         []string{etag},
			},
		}
		return res, nil
	})

	httpmock.RegisterResponder(http.MethodGet, forbiddenRawURL, httpmock.NewStringResponder(http.StatusForbidden, "forbidden"))
	httpmock.RegisterResponder(http.MethodGet, notfoundRawURL, httpmock.NewStringResponder(http.StatusNotFound, "not found"))
	httpmock.RegisterResponder(http.MethodGet, normalNotSupportRangeRawURL, httpmock.NewStringResponder(http.StatusOK, testContent))
	httpmock.RegisterResponder(http.MethodGet, errorRawURL, httpmock.NewErrorResponder(fmt.Errorf("error")))
}

func (suite *HTTPSourceClientTestSuite) TestNewHTTPSourceClient() {
	var sourceClient source.ResourceClient
	sourceClient = NewHTTPSourceClient()
	suite.Equal(_defaultHTTPClient, sourceClient.(*httpSourceClient).httpClient)
	suite.EqualValues(*_defaultHTTPClient, *sourceClient.(*httpSourceClient).httpClient)

	expectedHTTPClient := &http.Client{}
	sourceClient = NewHTTPSourceClient(WithHTTPClient(expectedHTTPClient))
	suite.Equal(expectedHTTPClient, sourceClient.(*httpSourceClient).httpClient)
	suite.EqualValues(*expectedHTTPClient, *sourceClient.(*httpSourceClient).httpClient)
}

func (suite *HTTPSourceClientTestSuite) TestHttpSourceClientDownloadWithResponseHeader() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	timeoutRequest, err := source.NewRequestWithContext(ctx, timeoutRawURL)
	suite.Nil(err)
	response, err := suite.DownloadWithResponseHeader(timeoutRequest)
	cancel()
	suite.NotNil(err)
	suite.Equal("Get \"http://timeout.com\": context deadline exceeded", err.Error())
	suite.Nil(response)

	tests := []struct {
		name       string
		request    *source.Request
		content    string
		expireInfo source.Header
		wantErr    error
	}{
		{
			name: "normal download",
			request: &source.Request{
				URL:    normalURL,
				Header: nil,
			},
			content: testContent,
			expireInfo: source.Header{
				source.LastModified: []string{lastModified},
				source.ETag:         []string{etag},
			},
			wantErr: nil,
		}, {
			name: "range download",
			request: &source.Request{
				URL:    normalURL,
				Header: source.RequestHeader{"Range": fmt.Sprintf("bytes=%s", "0-3")},
			},
			content: testContent[0:3],
			expireInfo: source.ResponseHeader{
				headers.LastModified: lastModified,
				headers.ETag:         etag,
			},
			wantErr: nil,
		}, {
			name: "not found download",
			request: &source.Request{
				URL: notfoundURL,
			},
			content:    "",
			expireInfo: nil,
			wantErr:    fmt.Errorf("unexpected status code: %d", http.StatusNotFound),
		}, {
			name: "error download",
			request: &source.Request{
				URL: errorURL,
			},
			content:    "",
			expireInfo: nil,
			wantErr: &url.Error{
				Op:  "Get",
				URL: errorRawURL,
				Err: fmt.Errorf("error"),
			},
		},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			response, err := suite.DownloadWithResponseHeader(tt.request)
			suite.Equal(tt.wantErr, err)
			if err != nil {
				return
			}
			bytes, err := ioutil.ReadAll(response.Body)
			suite.Nil(err)
			suite.Equal(tt.content, string(bytes))
			suite.Equal(tt.expireInfo, response.Header)
		})
	}
}

func (suite *HTTPSourceClientTestSuite) TestHttpSourceClientGetContentLength() {
	tests := []struct {
		name    string
		request *source.Request
		want    int64
		wantErr error
	}{
		{name: "support content length", args: args{ctx: context.Background(), url: normalURL, header: map[string]string{}}, want: int64(len(testContent)),
			wantErr: nil},
		{name: "not support content length", args: args{ctx: context.Background(), url: normalURL, header: source.RequestHeader{"Range": fmt.Sprintf("bytes=%s",
			"0-3")}}, want: 4,
			wantErr: nil},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			got, err := suite.GetContentLength(tt.request)
			suite.Equal(tt.wantErr, err)
			suite.Equal(tt.want, got)
		})
	}
}

func (suite *HTTPSourceClientTestSuite) TestHttpSourceClientIsExpired() {
	notExpireRequest := normalRequest.Clone(context.Background())
	notExpireRequest.Header.Add(source.LastModified, lastModified)
	notExpireRequest.Header.Add(source.ETag, etag)
	errorNotExpireRequest := errorRequest.Clone(context.Background())
	errorNotExpireRequest.Header.Add(source.LastModified, lastModified)
	errorNotExpireRequest.Header.Add(source.ETag, etag)
	expiredRequest := normalRequest.Clone(context.Background())

	tests := []struct {
		name    string
		request *source.Request
		want    bool
		wantErr bool
	}{
		{name: "not expire", request: notExpireRequest, want: false, wantErr: false},
		{name: "error not expire", request: errorNotExpireRequest, want: false, wantErr: true},
		{name: "expired", request: expiredRequest, want: true, wantErr: false},
	}
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			got, err := suite.IsExpired(tt.request)
			suite.Equal(tt.want, got)
			suite.Equal(tt.wantErr, err != nil)
		})
	}
}

func (suite *HTTPSourceClientTestSuite) TestHttpSourceClientIsSupportRange() {
	httpmock.RegisterResponder(http.MethodGet, timeoutURL, func(request *http.Request) (*http.Response, error) {
		time.Sleep(3 * time.Second)
		return httpmock.NewStringResponse(http.StatusOK, "ok"), nil
	})
	parent := context.Background()
	ctx, cancel := context.WithTimeout(parent, 1*time.Second)
	request, err := source.NewRequestWithContext(ctx, timeoutURL)
	suite.Nil(err)
	support, err := suite.IsSupportRange(request)
	cancel()
	suite.NotNil(err)
	suite.Equal("Get \"http://timeout.com\": context deadline exceeded", err.Error())
	suite.Equal(false, support)
	httpmock.RegisterResponder(http.MethodGet, normalRawURL, httpmock.NewStringResponder(http.StatusPartialContent, ""))
	httpmock.RegisterResponder(http.MethodGet, normalNotSupportRangeRawURL, httpmock.NewStringResponder(http.StatusOK, ""))
	httpmock.RegisterResponder(http.MethodGet, errorRawURL, httpmock.NewErrorResponder(fmt.Errorf("xxx")))

	supportRequest := normalRequest.Clone(context.Background())
	supportRequest.Header.Add("Range", fmt.Sprintf("bytes=%s", "0-3"))
	suite.Nil(err)
	notSupportRequest := normalNotSupportRangeRequest.Clone(context.Background())
	supportRequest.Header.Add("Range", fmt.Sprintf("bytes=%s", "0-3"))
	suite.Nil(err)
	tests := []struct {
		name    string
		request *source.Request
		want    bool
		wantErr bool
	}{
		{name: "support", request: supportRequest, want: true, wantErr: false},
		{name: "notSupport", request: notSupportRequest, want: false, wantErr: false},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			got, err := suite.IsSupportRange(tt.request)
			suite.Equal(tt.wantErr, err != nil)
			suite.Equal(tt.want, got)
		})
	}
}

func (suite *HTTPSourceClientTestSuite) TestHttpSourceClientDoRequest() {
	var testURL = "http://www.hackhttp.com"
	httpmock.RegisterResponder(http.MethodGet, testURL, httpmock.NewStringResponder(http.StatusOK, "ok"))
	request, err := source.NewRequest(testURL)
	suite.Nil(err)
	res, err := suite.ResourceClient.(*httpSourceClient).doRequest(http.MethodGet, request)
	suite.Nil(err)
	bytes, err := ioutil.ReadAll(res.Body)
	suite.Nil(err)
	suite.EqualValues("ok", string(bytes))
}
