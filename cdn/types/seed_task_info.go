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

package types

import (
	"net/url"
	"strings"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/net/urlutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
)

type SeedTask struct {
	// ID of the task
	ID string `json:"ID,omitempty"`

	// RawURL is the resource's URL which user uses dfget to download. The location of URL can be anywhere, LAN or WAN.
	// For image distribution, this is image layer's URL in image registry.
	// The resource url is provided by dfget command line parameter.
	RawURL string `json:"rawURL,omitempty"`

	// TaskURL is generated from rawURL. rawURL may contain some queries or parameter, dfget will filter some queries via
	// --filter parameter of dfget. The usage of it is that different rawURL may generate the same taskID.
	TaskURL string `json:"taskURL,omitempty"`

	// SourceFileLength is the length of the source file in bytes.
	SourceFileLength int64 `json:"sourceFileLength,omitempty"`

	// CdnFileLength is the length of the file stored on CDN
	CdnFileLength int64 `json:"cdnFileLength,omitempty"`

	// PieceSize is the size of pieces in bytes
	PieceSize int32 `json:"pieceSize,omitempty"`

	// UrlMeta is meta info of downloading request url
	UrlMeta *base.UrlMeta `json:"header,omitempty"`

	// CdnStatus is the status of the created task related to CDN functionality.
	//
	// Enum: [WAITING RUNNING FAILED SUCCESS SOURCE_ERROR]
	CdnStatus string `json:"cdnStatus,omitempty"`

	// PieceTotal is the total count of all pieces
	PieceTotal int32 `json:"pieceTotal,omitempty"`

	// RequestDigest is the digest of request which is provided by dfget cli
	RequestDigest string `json:"requestDigest,omitempty"`

	// SourceRealDigest when CDN finishes downloading file/image from the source location,
	// the md5 sum of the source file will be calculated as the value of the SourceRealDigest.
	// And it will be used to compare with RequestDigest value to check whether file is complete.
	SourceRealDigest string `json:"sourceRealDigest,omitempty"`

	// PieceMd5Sign Is the SHA256 signature of all pieces md5 signature
	PieceMd5Sign string `json:"pieceMd5Sign,omitempty"`

	logger *logger.SugaredLoggerOnWith
}

const (
	UnKnownSourceFileLen = -100
)

func NewSeedTask(taskID string, rawURL string, urlMeta *base.UrlMeta) *SeedTask {
	taskURL := rawURL
	if urlMeta != nil && !stringutils.IsEmpty(urlMeta.Filter) {
		taskURL = urlutils.FilterURLParam(rawURL, strings.Split(urlMeta.Filter, "&"))
	}
	return &SeedTask{
		ID:            taskID,
		RawURL:        rawURL,
		TaskURL:       taskURL,
		UrlMeta:       urlMeta,
		RequestDigest: urlMeta.Digest,

		CdnStatus:        TaskInfoCdnStatusWaiting,
		SourceFileLength: UnKnownSourceFileLen,
		logger:           logger.WithTaskID(taskID),
	}
}

// IsSuccess determines that whether the CDNStatus is success.
func (task *SeedTask) IsSuccess() bool {
	return task.CdnStatus == TaskInfoCdnStatusSuccess
}

// IsFrozen if task status is frozen
func (task *SeedTask) IsFrozen() bool {
	return task.CdnStatus == TaskInfoCdnStatusFailed || task.CdnStatus == TaskInfoCdnStatusWaiting || task.CdnStatus == TaskInfoCdnStatusSourceError
}

// IsWait if task status is wait
func (task *SeedTask) IsWait() bool {
	return task.CdnStatus == TaskInfoCdnStatusWaiting
}

// IsError if task status if fail
func (task *SeedTask) IsError() bool {
	return task.CdnStatus == TaskInfoCdnStatusFailed || task.CdnStatus == TaskInfoCdnStatusSourceError
}

func (task *SeedTask) IsDone() bool {
	return task.CdnStatus == TaskInfoCdnStatusFailed || task.CdnStatus == TaskInfoCdnStatusSuccess || task.CdnStatus == TaskInfoCdnStatusSourceError
}

func (task *SeedTask) UpdateStatus(cdnStatus string) {
	task.CdnStatus = cdnStatus
}

func (task *SeedTask) UpdateTaskInfo(cdnStatus, realDigest, pieceMd5Sign string, sourceFileLength, cdnFileLength int64) {
	task.CdnStatus = cdnStatus
	task.PieceMd5Sign = pieceMd5Sign
	task.SourceRealDigest = realDigest
	task.SourceFileLength = sourceFileLength
	task.CdnFileLength = cdnFileLength
}

func (task *SeedTask) Log() *logger.SugaredLoggerOnWith {
	return task.logger
}

func (task *SeedTask) GetSourceRequest() *source.Request {
	return &source.Request{
		URL: url.Parse(task.RawURL),
		Header: source.RequestHeader{
			Header: nil,
		},
	}
}

const (

	// TaskInfoCdnStatusWaiting captures enum value "WAITING"
	TaskInfoCdnStatusWaiting string = "WAITING"

	// TaskInfoCdnStatusRunning captures enum value "RUNNING"
	TaskInfoCdnStatusRunning string = "RUNNING"

	// TaskInfoCdnStatusFailed captures enum value "FAILED"
	TaskInfoCdnStatusFailed string = "FAILED"

	// TaskInfoCdnStatusSuccess captures enum value "SUCCESS"
	TaskInfoCdnStatusSuccess string = "SUCCESS"

	// TaskInfoCdnStatusSourceError captures enum value "SOURCE_ERROR"
	TaskInfoCdnStatusSourceError string = "SOURCE_ERROR"
)
