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

	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/supervisor/cdn/storage"
	"d7y.io/dragonfly/v2/cdn/types"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type reporter struct {
	progressManager supervisor.SeedProgressManager
}

const (
	CacheReport      = "cache"
	DownloaderReport = "download"
)

func newReporter(publisher supervisor.SeedProgressManager) *reporter {
	return &reporter{
		progressManager: publisher,
	}
}

// reportDetectResult report detect cache result
func (re *reporter) reportDetectResult(ctx context.Context, taskID string, detectResult *cacheResult) error {
	// report cache pieces status
	if detectResult != nil && detectResult.pieceMetaRecords != nil {
		for _, record := range detectResult.pieceMetaRecords {
			if err := re.reportPieceMetaRecord(ctx, taskID, record, CacheReport); err != nil {
				return errors.Wrapf(err, "publish pieceMetaRecord: %v, seedPiece: %v", record,
					convertPieceMeta2SeedPiece(record))
			}
		}
	}
	return nil
}

// reportPieceMetaRecord report piece meta record
func (re *reporter) reportPieceMetaRecord(ctx context.Context, taskID string, record *storage.PieceMetaRecord, from string) error {
	// report cache piece status
	logger.DownloaderLogger.Info(taskID,
		zap.Int32("pieceNum", record.PieceNum),
		zap.String("md5", record.Md5),
		zap.String("from", from))
	return re.progressManager.PublishPiece(ctx, taskID, convertPieceMeta2SeedPiece(record))
}

/*
	helper functions
*/
func convertPieceMeta2SeedPiece(record *storage.PieceMetaRecord) *types.SeedPiece {
	return &types.SeedPiece{
		PieceStyle:  record.PieceStyle,
		PieceNum:    record.PieceNum,
		PieceMd5:    record.Md5,
		PieceRange:  record.Range,
		OriginRange: record.OriginRange,
		PieceLen:    record.PieceLen,
	}
}
