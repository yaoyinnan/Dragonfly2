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

//go:generate mockgen -destination ../mocks/proxy/mock_proxy_manager.go -package progress d7y.io/dragonfly/v2/cdn/supervisor/proxy Manager

package proxy

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"sync"
	"time"

	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"d7y.io/dragonfly/v2/pkg/source"
)

// Manager as an interface defines all operations about proxy
type Manager interface {

	// AddProxy add a proxy config
	AddProxy(urlReg string, proxyHost string) error

	// RemoveProxy remove a proxy config
	RemoveProxy(urlReg string)

	// ListProxies list
	ListProxies() map[string]string

	// TryProxy try proxy
	TryProxy(ctx context.Context, seedTask *task.SeedTask) (*source.Request, bool)
}

var _ Manager = (*manager)(nil)

type manager struct {
	config  Config
	mu      sync.RWMutex
	proxies map[string]*proxyItem
}

// NewManager returns a new Manager Object.
func NewManager(config Config) (Manager, error) {
	config.rules = append(config.rules, &Proxy{
		Regx:      "https://download.jetbrains.com.cn/go/goland-2021.3.dmg",
		ProxyHost: "localhost:8080",
	})
	proxies := make(map[string]*proxyItem)
	for _, rule := range config.rules {
		compiled, err := regexp.Compile(rule.Regx)
		if err != nil {
			return nil, err
		}
		proxies[rule.Regx] = &proxyItem{
			regexp:    compiled,
			proxyHost: rule.ProxyHost,
		}
	}
	manager := &manager{
		config:  config,
		proxies: proxies,
	}
	return manager, nil
}

func (m *manager) AddProxy(urlReg string, proxyHost string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	compiledReg, err := regexp.Compile(urlReg)
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("tcp", proxyHost, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	m.proxies[urlReg] = &proxyItem{
		regexp:    compiledReg,
		proxyHost: proxyHost,
	}
	return nil
}

func (m *manager) RemoveProxy(urlReg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.proxies, urlReg)
}

func (m *manager) ListProxies() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	proxyMap := make(map[string]string)
	for s, item := range m.proxies {
		proxyMap[s] = item.proxyHost
	}
	return proxyMap
}

func (m *manager) TryProxy(ctx context.Context, seedTask *task.SeedTask) (*source.Request, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, item := range m.proxies {
		if item.regexp.MatchString(seedTask.RawURL) {
			proxyURL := fmt.Sprintf("proxy://%s/proxy", item.proxyHost)
			request, err := source.NewRequestWithContext(ctx, proxyURL, seedTask.Header)
			// set origin URL
			request.Header.Set(source.Referer, seedTask.RawURL)
			request.Header.Set("taskID", seedTask.ID)
			seedTask.Log().Infof("%s proxy request %s at Range %s", item.proxyHost, seedTask.RawURL, seedTask.Range)
			if err != nil {
				continue
			}
			return request, true
		}
	}
	return nil, false
}
