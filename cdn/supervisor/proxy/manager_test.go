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

package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"d7y.io/dragonfly/v2/pkg/source"
	sourcemock "d7y.io/dragonfly/v2/pkg/source/mock"
	"d7y.io/dragonfly/v2/pkg/source/proxyprotocol"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyManager(t *testing.T) {
	ctl := gomock.NewController(t)
	sourceClient := sourcemock.NewMockResourceClient(ctl)
	source.UnRegister("proxy")
	source.Register("proxy", sourceClient, proxyprotocol.Adapter)
	defer source.UnRegister("proxy")
	//sourceClient.EXPECT().GetContentLength(source.RequestEq(testTask.RawURL)).Return(int64(1024*1024*500+1000), nil).Times(1)
	manager, err := newManager(Config{})
	assert.Nil(t, err)
	// add a unreachable proxyHost
	err = manager.AddProxy("https://www.dragonfly.com", "127.0.0.1:8888")
	require.EqualError(t, err, "dial tcp 127.0.0.1:8888: connect: connection refused")
	// add a reachable proxyHost
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
	}))
	defer server.Close()
	err = manager.AddProxy("https://www.dragonfly.com.+", server.Listener.Addr().String())
	require.NoError(t, err)
	err = manager.AddProxy("https://www.test.com.+", server.Listener.Addr().String())
	require.NoError(t, err)
	proxies := manager.ListProxies()
	require.Equal(t, map[string]string{
		"https://www.test.com.+":      server.Listener.Addr().String(),
		"https://www.dragonfly.com.+": server.Listener.Addr().String(),
	}, proxies)
	// remove a nonexistent proxy
	manager.RemoveProxy("https://www.nonexistent.com")
	proxies = manager.ListProxies()
	require.Equal(t, map[string]string{
		"https://www.test.com.+":      server.Listener.Addr().String(),
		"https://www.dragonfly.com.+": server.Listener.Addr().String(),
	}, proxies)
	// remove test proxy
	manager.RemoveProxy("https://www.test.com.+")
	proxies = manager.ListProxies()
	require.Equal(t, map[string]string{
		"https://www.dragonfly.com.+": server.Listener.Addr().String(),
	}, proxies)
	proxyTask := &task.SeedTask{
		ID:      "testProxyTask",
		RawURL:  "https://www.dragonfly.com/test",
		TaskURL: "https://www.dragonfly.com/test",
	}
	request, ok := manager.TryProxy(context.Background(), proxyTask)
	require.True(t, ok)
	proxyUrl, _ := url.Parse(fmt.Sprintf("proxy://%s/proxy", server.Listener.Addr().String()))
	require.Equal(t, proxyUrl, request.URL)
}
