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

package rpcserver

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"

	"d7y.io/dragonfly/v2/cdn/config"
	"d7y.io/dragonfly/v2/cdn/supervisor"
	"d7y.io/dragonfly/v2/cdn/supervisor/task"
	"d7y.io/dragonfly/v2/internal/dferrors"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	"d7y.io/dragonfly/v2/internal/idgen"
	"d7y.io/dragonfly/v2/pkg/rpc/base"
	"d7y.io/dragonfly/v2/pkg/rpc/cdnsystem"
	cdnserver "d7y.io/dragonfly/v2/pkg/rpc/cdnsystem/server"
	"d7y.io/dragonfly/v2/pkg/util/hostutils"
	"d7y.io/dragonfly/v2/pkg/util/net/urlutils"
	"d7y.io/dragonfly/v2/pkg/util/rangeutils"
	"d7y.io/dragonfly/v2/pkg/util/stringutils"
)

var tracer = otel.Tracer("cdn-server")

type server struct {
	rpcServer  *grpc.Server
	cfg        *config.Config
	cdnService supervisor.CDNService
}

// New returns a new Manager Object.
func New(cfg *config.Config, cdnService supervisor.CDNService, opts ...grpc.ServerOption) (*grpc.Server, error) {
	svr := &server{
		cdnService: cdnService,
		cfg:        cfg,
	}
	svr.rpcServer = cdnserver.New(svr, opts...)
	return svr.rpcServer, nil
}

// checkSeedRequestParams check the params of SeedRequest.
func checkSeedRequestParams(req *cdnsystem.SeedRequest) error {
	if stringutils.IsBlank(req.TaskId) {
		return errors.New("taskID is empty")
	}
	if !urlutils.IsValidURL(req.Url) {
		return errors.Errorf("resource url: %s is invalid", req.Url)
	}
	if req.UrlMeta != nil && stringutils.IsBlank(req.UrlMeta.Range) {
		// todo check range is valid and source support range
		_, err := rangeutils.GetRange(req.UrlMeta.Range)
		if err != nil {
			return errors.Errorf("request range: %s is valid", req.UrlMeta.Range)
		}
	}
	return nil
}

func (css *server) ObtainSeeds(ctx context.Context, req *cdnsystem.SeedRequest, psc chan<- *cdnsystem.PieceSeed) (err error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanObtainSeeds, trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()
	span.SetAttributes(config.AttributeObtainSeedsRequest.String(req.String()))
	span.SetAttributes(config.AttributeTaskID.String(req.TaskId))
	logger.Infof("obtain seeds request: %+v", req)
	defer func() {
		if r := recover(); r != nil {
			err = dferrors.Newf(base.Code_UnknownError, "obtain task(%s) seeds encounter an panic: %v", req.TaskId, r)
			span.RecordError(err)
			logger.WithTaskID(req.TaskId).Errorf("%v", err)
		}
		logger.Infof("seeds task %s result success: %t", req.TaskId, err == nil)
	}()
	if err := checkSeedRequestParams(req); err != nil {
		err = dferrors.Newf(base.Code_BadRequest, "bad seed request for task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return err
	}
	// register seed task
	pieceChan, err := css.cdnService.RegisterSeedTask(ctx, task.NewSeedTask(req.TaskId, req.Url, req.UrlMeta))
	if err != nil {
		if supervisor.IsResourcesLacked(err) {
			err = dferrors.Newf(base.Code_ResourceLacked, "resources lacked for task(%s): %v", req.TaskId, err)
			span.RecordError(err)
			return err
		}
		err = dferrors.Newf(base.Code_CDNTaskRegistryFail, "failed to register seed task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return err
	}
	peerID := idgen.CDNPeerID(css.cfg.AdvertiseIP)
	hostID := idgen.CDNHostID(hostutils.FQDNHostname, int32(css.cfg.ListenPort))
	for piece := range pieceChan {
		psc <- &cdnsystem.PieceSeed{
			PeerId:   peerID,
			HostUuid: hostID,
			PieceInfo: &base.PieceInfo{
				PieceNum:    piece.PieceNum,
				RangeStart:  uint64(piece.PieceRange.StartIndex),
				RangeSize:   piece.PieceLen,
				PieceMd5:    piece.PieceMd5,
				PieceOffset: uint64(piece.OriginRange.StartIndex),
				PieceStyle:  piece.PieceStyle,
			},
			Done:            false,
			ContentLength:   task.UnKnownSourceFileLen,
			TotalPieceCount: task.UnknownTotalPieceCount,
		}
	}
	seedTask, err := css.cdnService.GetSeedTask(req.TaskId)
	if err != nil {
		err = dferrors.Newf(base.Code_CDNError, "failed to get task(%s): %v", req.TaskId, err)
		if task.IsTaskNotFound(err) {
			err = dferrors.Newf(base.Code_CDNTaskNotFound, "failed to get task(%s): %v", req.TaskId, err)
			span.RecordError(err)
			return err
		}
		err = dferrors.Newf(base.Code_CDNError, "failed to get task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return err
	}
	if !seedTask.IsSuccess() {
		err = dferrors.Newf(base.Code_CDNTaskDownloadFail, "task(%s) status error , status: %s", req.TaskId, seedTask.CdnStatus)
		span.RecordError(err)
		return err
	}
	psc <- &cdnsystem.PieceSeed{
		PeerId:          peerID,
		HostUuid:        hostID,
		Done:            true,
		ContentLength:   seedTask.SourceFileLength,
		TotalPieceCount: seedTask.TotalPieceCount,
	}
	return nil
}

func (css *server) GetPieceTasks(ctx context.Context, req *base.PieceTaskRequest) (piecePacket *base.PiecePacket, err error) {
	var span trace.Span
	ctx, span = tracer.Start(ctx, config.SpanGetPieceTasks, trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()
	span.SetAttributes(config.AttributeGetPieceTasksRequest.String(req.String()))
	span.SetAttributes(config.AttributeTaskID.String(req.TaskId))
	logger.Infof("get piece tasks: %+v", req)
	defer func() {
		if r := recover(); r != nil {
			err = dferrors.Newf(base.Code_UnknownError, "get task(%s) piece tasks encounter an panic: %v", req.TaskId, r)
			span.RecordError(err)
			logger.WithTaskID(req.TaskId).Errorf("get piece tasks failed: %v", err)
		}
		logger.WithTaskID(req.TaskId).Infof("get piece tasks result success: %t", err == nil)
	}()
	if err := checkPieceTasksRequestParams(req); err != nil {
		err = dferrors.Newf(base.Code_BadRequest, "failed to validate seed request for task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return nil, err
	}
	seedTask, err := css.cdnService.GetSeedTask(req.TaskId)
	if err != nil {
		if task.IsTaskNotFound(err) {
			err = dferrors.Newf(base.Code_CDNTaskNotFound, "failed to get task(%s): %v", req.TaskId, err)
			span.RecordError(err)
			return nil, err
		}
		err = dferrors.Newf(base.Code_CDNError, "failed to get task(%s): %v", req.TaskId, err)
		span.RecordError(err)
		return nil, err
	}
	if seedTask.IsError() {
		err = dferrors.Newf(base.Code_CDNTaskDownloadFail, "task(%s) status is FAIL, cdnStatus: %s", seedTask.ID, seedTask.CdnStatus)
		span.RecordError(err)
		return nil, err
	}
	pieces, err := css.cdnService.GetPieces(req.TaskId)
	if err != nil {
		err = dferrors.Newf(base.Code_CDNError, "failed to get pieces of task(%s) from cdn: %v", seedTask.ID, err)
		span.RecordError(err)
		return nil, err
	}
	pieceInfos := make([]*base.PieceInfo, 0)
	var count int32 = 0
	for _, piece := range pieces {
		if piece.PieceNum >= req.StartNum && (count < req.Limit || req.Limit <= 0) {
			p := &base.PieceInfo{
				PieceNum:    piece.PieceNum,
				RangeStart:  uint64(piece.PieceRange.StartIndex),
				RangeSize:   piece.PieceLen,
				PieceMd5:    piece.PieceMd5,
				PieceOffset: uint64(piece.OriginRange.StartIndex),
				PieceStyle:  piece.PieceStyle,
			}
			pieceInfos = append(pieceInfos, p)
			count++
		}
	}
	pp := &base.PiecePacket{
		TaskId:        req.TaskId,
		DstPid:        req.DstPid,
		DstAddr:       fmt.Sprintf("%s:%d", css.cfg.AdvertiseIP, css.cfg.DownloadPort),
		PieceInfos:    pieceInfos,
		TotalPiece:    seedTask.TotalPieceCount,
		ContentLength: seedTask.SourceFileLength,
		PieceMd5Sign:  seedTask.PieceMd5Sign,
	}
	span.SetAttributes(config.AttributePiecePacketResult.String(pp.String()))
	return pp, nil
}

func checkPieceTasksRequestParams(req *base.PieceTaskRequest) error {
	if stringutils.IsBlank(req.TaskId) {
		return errors.New("taskID is empty")
	}
	if stringutils.IsBlank(req.SrcPid) {
		return errors.New("src peerID is empty")
	}
	if req.StartNum < 0 {
		return errors.Errorf("invalid starNum %d", req.StartNum)
	}
	if req.Limit < 0 {
		return errors.Errorf("invalid limit %d", req.Limit)
	}
	return nil
}
