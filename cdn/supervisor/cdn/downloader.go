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
	"github.com/pkg/errors"
)

func (cm *Manager) download(ctx context.Context, task *types.SeedTask, breakPoint int64) (io.ReadCloser, error) {
	if breakPoint > 0 {
		breakRange, err := getBreakRange(breakPoint, task.SourceFileLength)
		if err != nil {
			return nil, errors.Wrapf(err, "calculate the breakRange")
		}
	}
	task.Log().Infof("start download url %s at range: %d-%d: with header: %+v", task.RawURL, breakPoint,
		task.SourceFileLength, task.Range)
	reader, responseHeader, err := source.DownloadWithResponseHeader(ctx, task.RawURL, headers)
	// update Expire info
	if err == nil {
		expireInfo := map[string]string{
			source.LastModified: responseHeader[source.LastModified],
			source.ETag:         responseHeader.Get(source.ETag),
		}
		cm.updateExpireInfo(task.ID, expireInfo)
	}
	return reader, err
}

func getBreakRange(breakPoint int64, sourceFileLength int64) (*rangeutils.Range, error) {
	if breakPoint <= 0 {
		return nil, fmt.Errorf("breakPoint is illegal for value: %d", breakPoint)
	}
	if sourceFileLength <= 0 {
		return nil, fmt.Errorf("sourceFileLength is illegal for value: %d", sourceFileLength)
	}
	end := sourceFileLength - 1
	if breakPoint > end {
		return nil, fmt.Errorf("start: %d is larger than end: %d", breakPoint, end)

	}
	return &rangeutils.Range{
		StartIndex: uint64(breakPoint),
		EndIndex:   uint64(end),
	}, nil
}
