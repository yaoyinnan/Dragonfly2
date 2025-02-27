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

//go:generate mockgen -destination mocks/client_mock.go -source client.go -package mocks

package client

import (
	"context"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"

	commonv1 "d7y.io/api/pkg/apis/common/v1"
	schedulerv1 "d7y.io/api/pkg/apis/scheduler/v1"

	"d7y.io/dragonfly/v2/client/config"
	logger "d7y.io/dragonfly/v2/internal/dflog"
	pkgbalancer "d7y.io/dragonfly/v2/pkg/balancer"
	"d7y.io/dragonfly/v2/pkg/resolver"
	"d7y.io/dragonfly/v2/pkg/rpc"
	"d7y.io/dragonfly/v2/pkg/rpc/common"
)

const (
	// contextTimeout is timeout of grpc invoke.
	contextTimeout = 2 * time.Minute

	// maxRetries is maximum number of retries.
	maxRetries = 3

	// backoffWaitBetween is waiting for a fixed period of
	// time between calls in backoff linear.
	backoffWaitBetween = 500 * time.Millisecond
)

func GetClient(ctx context.Context, dynconfig config.Dynconfig, opts ...grpc.DialOption) (Client, error) {
	// Register resolver and balancer.
	resolver.RegisterScheduler(dynconfig)
	builder, pickerBuilder := pkgbalancer.NewConsistentHashingBuilder()
	balancer.Register(builder)

	conn, err := grpc.DialContext(
		ctx,
		resolver.SchedulerVirtualTarget,
		append([]grpc.DialOption{
			grpc.WithDefaultServiceConfig(pkgbalancer.BalancerServiceConfig),
			grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
				rpc.ConvertErrorUnaryClientInterceptor,
				otelgrpc.UnaryClientInterceptor(),
				grpc_prometheus.UnaryClientInterceptor,
				grpc_zap.UnaryClientInterceptor(logger.GrpcLogger.Desugar()),
				grpc_retry.UnaryClientInterceptor(
					grpc_retry.WithMax(maxRetries),
					grpc_retry.WithBackoff(grpc_retry.BackoffLinear(backoffWaitBetween)),
				),
				rpc.RefresherUnaryClientInterceptor(dynconfig),
			)),
			grpc.WithStreamInterceptor(grpc_middleware.ChainStreamClient(
				rpc.ConvertErrorStreamClientInterceptor,
				otelgrpc.StreamClientInterceptor(),
				grpc_prometheus.StreamClientInterceptor,
				grpc_zap.StreamClientInterceptor(logger.GrpcLogger.Desugar()),
				rpc.RefresherStreamClientInterceptor(dynconfig),
			)),
		}, opts...)...,
	)
	if err != nil {
		return nil, err
	}

	return &client{
		SchedulerClient:                schedulerv1.NewSchedulerClient(conn),
		ClientConn:                     conn,
		Dynconfig:                      dynconfig,
		dialOptions:                    opts,
		ConsistentHashingPickerBuilder: pickerBuilder,
	}, nil
}

// Client is the interface for grpc client.
type Client interface {
	// RegisterPeerTask registers a peer into task.
	RegisterPeerTask(context.Context, *schedulerv1.PeerTaskRequest, ...grpc.CallOption) (*schedulerv1.RegisterResult, error)

	// ReportPieceResult reports piece results and receives peer packets.
	ReportPieceResult(context.Context, *schedulerv1.PeerTaskRequest, ...grpc.CallOption) (schedulerv1.Scheduler_ReportPieceResultClient, error)

	// ReportPeerResult reports downloading result for the peer.
	ReportPeerResult(context.Context, *schedulerv1.PeerResult, ...grpc.CallOption) error

	// A peer announces that it has the announced task to other peers.
	AnnounceTask(context.Context, *schedulerv1.AnnounceTaskRequest, ...grpc.CallOption) error

	// Checks if any peer has the given task.
	StatTask(context.Context, *schedulerv1.StatTaskRequest, ...grpc.CallOption) (*schedulerv1.Task, error)

	// LeaveTask releases peer in scheduler.
	LeaveTask(context.Context, *schedulerv1.PeerTarget, ...grpc.CallOption) error

	// AnnounceHost announces host to scheduler.
	AnnounceHost(context.Context, *schedulerv1.AnnounceHostRequest, ...grpc.CallOption) error

	// LeaveHost releases host in scheduler.
	LeaveHost(context.Context, *schedulerv1.LeaveHostRequest, ...grpc.CallOption) error

	// Close tears down the ClientConn and all underlying connections.
	Close() error
}

// client provides scheduler grpc function.
type client struct {
	schedulerv1.SchedulerClient
	*grpc.ClientConn
	config.Dynconfig
	dialOptions []grpc.DialOption
	*pkgbalancer.ConsistentHashingPickerBuilder
}

// RegisterPeerTask registers a peer into task.
func (c *client) RegisterPeerTask(ctx context.Context, req *schedulerv1.PeerTaskRequest, opts ...grpc.CallOption) (*schedulerv1.RegisterResult, error) {
	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

	return c.SchedulerClient.RegisterPeerTask(
		context.WithValue(ctx, pkgbalancer.ContextKey, req.TaskId),
		req,
		opts...,
	)
}

// ReportPieceResult reports piece results and receives peer packets.
func (c *client) ReportPieceResult(ctx context.Context, req *schedulerv1.PeerTaskRequest, opts ...grpc.CallOption) (schedulerv1.Scheduler_ReportPieceResultClient, error) {
	stream, err := c.SchedulerClient.ReportPieceResult(
		context.WithValue(ctx, pkgbalancer.ContextKey, req.TaskId),
		opts...,
	)
	if err != nil {
		return nil, err
	}

	// Send begin of piece.
	return stream, stream.Send(&schedulerv1.PieceResult{
		TaskId: req.TaskId,
		SrcPid: req.PeerId,
		PieceInfo: &commonv1.PieceInfo{
			PieceNum: common.BeginOfPiece,
		},
	})
}

// ReportPeerResult reports downloading result for the peer.
func (c *client) ReportPeerResult(ctx context.Context, req *schedulerv1.PeerResult, opts ...grpc.CallOption) error {
	_, err := c.SchedulerClient.ReportPeerResult(
		context.WithValue(ctx, pkgbalancer.ContextKey, req.TaskId),
		req,
		opts...,
	)

	return err
}

// A peer announces that it has the announced task to other peers.
func (c *client) AnnounceTask(ctx context.Context, req *schedulerv1.AnnounceTaskRequest, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

	_, err := c.SchedulerClient.AnnounceTask(
		context.WithValue(ctx, pkgbalancer.ContextKey, req.TaskId),
		req,
		opts...,
	)

	return err
}

// Checks if any peer has the given task.
func (c *client) StatTask(ctx context.Context, req *schedulerv1.StatTaskRequest, opts ...grpc.CallOption) (*schedulerv1.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

	return c.SchedulerClient.StatTask(
		context.WithValue(ctx, pkgbalancer.ContextKey, req.TaskId),
		req,
		opts...,
	)
}

// LeaveTask releases peer in scheduler.
func (c *client) LeaveTask(ctx context.Context, req *schedulerv1.PeerTarget, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

	_, err := c.SchedulerClient.LeaveTask(
		context.WithValue(ctx, pkgbalancer.ContextKey, req.TaskId),
		req,
		opts...,
	)

	return err
}

// AnnounceHost announces host to scheduler.
func (c *client) AnnounceHost(ctx context.Context, req *schedulerv1.AnnounceHostRequest, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

	circle, err := c.GetCircle()
	if err != nil {
		return err
	}

	eg, _ := errgroup.WithContext(ctx)
	for _, virtualTaskID := range circle {
		virtualTaskID := virtualTaskID
		eg.Go(func() error {
			if _, err := c.SchedulerClient.AnnounceHost(
				context.WithValue(ctx, pkgbalancer.ContextKey, virtualTaskID),
				req,
				opts...,
			); err != nil {
				return err
			}

			return nil
		})
	}

	return eg.Wait()
}

// LeaveHost releases host in all schedulers.
func (c *client) LeaveHost(ctx context.Context, req *schedulerv1.LeaveHostRequest, opts ...grpc.CallOption) error {
	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

	circle, err := c.GetCircle()
	if err != nil {
		return err
	}
	logger.Infof("leave host circle is %#v", circle)

	eg, _ := errgroup.WithContext(ctx)
	for _, virtualTaskID := range circle {
		virtualTaskID := virtualTaskID
		eg.Go(func() error {
			if _, err := c.SchedulerClient.LeaveHost(
				context.WithValue(ctx, pkgbalancer.ContextKey, virtualTaskID),
				req,
				opts...,
			); err != nil {
				return err
			}

			return nil
		})
	}

	return eg.Wait()
}
