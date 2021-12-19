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

package metrics

import (
	"d7y.io/dragonfly/v2/internal/constants"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Variables declared for metrics.
var (
	DownloadCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: constants.MetricsNamespace,
		Subsystem: constants.CDNMetricsName,
		Name:      "download_total",
		Help:      "Counter of the number of the downloading.",
	})

	DownloadFailureCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: constants.MetricsNamespace,
		Subsystem: constants.CDNMetricsName,
		Name:      "download_failure_total",
		Help:      "Counter of the number of failed of the downloading.",
	})

	DownloadTraffic = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: constants.MetricsNamespace,
		Subsystem: constants.CDNMetricsName,
		Name:      "download_traffic",
		Help:      "Counter of the number of download traffic.",
	})

	ConcurrentDownloadGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: constants.MetricsNamespace,
		Subsystem: constants.CDNMetricsName,
		Name:      "concurrent_download_total",
		Help:      "Gauger of the number of concurrent of the downloading.",
	})
)
