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

package cdn

import (
	"context"
	"fmt"
	"io"

	"d7y.io/dragonfly/v2/cdn/types"
	"d7y.io/dragonfly/v2/pkg/source"
	"d7y.io/dragonfly/v2/pkg/util/rangeutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
	"github.com/pkg/errors"
)

func (cm *Manager) download(ctx context.Context, task *types.SeedTask, breakPoint int64) (io.ReadCloser, error) {
	var err error
	breakRange := task.Range
	if breakPoint > 0 {
		breakRange, err = getBreakRange(breakPoint, task.SourceFileLength, task.Range)
		if err != nil {
			return nil, errors.Wrapf(err, "calculate the breakRange")
		}
	}
	task.Log().Infof("start download url %s at range: %d-%d: with header: %+v", task.RawURL, breakPoint,
		task.SourceFileLength, task.Range)
	downloadRequest, err := source.NewRequestWithHeader(task.RawURL, task.Header)
	if err != nil {
		return nil, errors.Wrap(err, "create download request")
	}
	if stringutils.IsBlank(breakRange) {
		downloadRequest.Header.Add(source.Range, breakRange)
	}
	resp, err := source.DownloadWithResponseHeader(downloadRequest)
	// update Expire info
	if err == nil {
		expireInfo := map[string]string{
			source.LastModified: resp.Header.Get(source.LastModified),
			source.ETag:         resp.Header.Get(source.ETag),
		}
		cm.updateExpireInfo(task.ID, expireInfo)
	}
	return resp.Body, err
}

func getBreakRange(breakPoint int64, sourceFileLength int64, taskRange string) (string, error) {
	var sourceStartIndex int64 = 0
	var sourceEndIndex int64 = 0
	if stringutils.IsBlank(taskRange) {
		reqRange, err := rangeutils.ParseRange(taskRange)
		if err != nil {
			return "", errors.Wrap(err, "parse request range")
		}
		sourceStartIndex = int64(reqRange.StartIndex)
		sourceEndIndex = int64(reqRange.EndIndex)
	}

	if breakPoint <= 0 {
		return "", errors.Errorf("breakPoint is illegal, breakPoint: %d", breakPoint)
	}
	start := breakPoint + sourceStartIndex
	if sourceFileLength < 0 {
		if sourceEndIndex > 0 {
			return fmt.Sprintf("%d-%d", start, sourceEndIndex), nil
		}
		return fmt.Sprintf("%d-", start), nil
	}
	end := sourceFileLength - 1 + sourceStartIndex
	if breakPoint > end {
		return "", fmt.Errorf("start: %d is larger than end: %d", breakPoint, end)
	}
	return fmt.Sprintf("%d-%d", start, end), nil
}
