// Copyright (c) 2025 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/uber-go/tally"
	"github.com/uber/submitqueue/core/errs"
	"github.com/uber/submitqueue/core/metrics"
	"github.com/uber/submitqueue/pushqueue/entity"
	"github.com/uber/submitqueue/pushqueue/extension/landqueue"
	"github.com/uber/submitqueue/pushqueue/extension/vcs"
	pb "github.com/uber/submitqueue/pushqueue/gateway/protopb"
	"go.uber.org/zap"
)

// LandController handles land and mergeability business logic for the
// pushqueue gateway.
type LandController struct {
	logger       *zap.SugaredLogger
	metricsScope tally.Scope
	vcs          vcs.VCS
	queue        landqueue.Queue
}

// NewLandController creates a new instance of the pushqueue land controller.
func NewLandController(logger *zap.SugaredLogger, scope tally.Scope, v vcs.VCS, q landqueue.Queue) *LandController {
	return &LandController{
		logger:       logger.Named("land_controller"),
		metricsScope: scope.SubScope("land_controller"),
		vcs:          v,
		queue:        q,
	}
}

// Land enqueues items for landing, waits for head-of-queue, pushes via VCS,
// and finalizes. Preparation is handled by the queue's Preparer.
func (c *LandController) Land(ctx context.Context, req *pb.LandRequest) (resp *pb.LandResponse, retErr error) {
	const opName = "land"

	op := metrics.Begin(c.metricsScope, opName)
	defer func() { op.Complete(retErr) }()

	target := entity.QueueTarget{
		Name:    req.Queue.GetName(),
		Address: req.Queue.GetAddress(),
		Target:  req.Queue.GetTarget(),
	}

	items := make([]entity.LandItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, entity.LandItem{
			URIs:     item.GetUris(),
			Strategy: resolveStrategy(item.GetStrategy()),
		})
	}

	if err := c.queue.Enqueue(ctx, target, items); err != nil {
		return nil, fmt.Errorf("enqueue: %w", err)
	}
	defer c.queue.Dequeue(ctx, target)

	if err := c.queue.Wait(ctx, target); err != nil {
		return nil, fmt.Errorf("wait: %w", err)
	}

	result, pushErr := c.vcs.Push(ctx, target, items)
	switch {
	case pushErr == nil:
	case errors.Is(pushErr, vcs.ErrConflict):
		return nil, errs.NewUserError(fmt.Errorf("conflict: %w", pushErr))
	case errors.Is(pushErr, vcs.ErrStaleHead):
		return nil, errs.NewRetryableError(fmt.Errorf("stale head: %w", pushErr))
	default:
		return nil, fmt.Errorf("push: %w", pushErr)
	}

	resp = &pb.LandResponse{
		Success:  true,
		Outcomes: make([]*pb.ItemOutcome, 0, len(result.Outcomes)),
	}
	for _, o := range result.Outcomes {
		resp.Outcomes = append(resp.Outcomes, &pb.ItemOutcome{
			Status:      outcomeStatusToProto(o.Status),
			RevisionIds: o.RevisionIDs,
		})
	}

	if err := c.vcs.Finalize(ctx, target, items); err != nil {
		resp.FinalizeError = err.Error()
	}

	return resp, nil
}

// CheckMergeability checks whether items can be landed on the target.
func (c *LandController) CheckMergeability(ctx context.Context, req *pb.CheckMergeabilityRequest) (resp *pb.CheckMergeabilityResponse, retErr error) {
	const opName = "check_mergeability"

	op := metrics.Begin(c.metricsScope, opName)
	defer func() { op.Complete(retErr) }()

	target := entity.QueueTarget{
		Name:    req.Queue.GetName(),
		Address: req.Queue.GetAddress(),
		Target:  req.Queue.GetTarget(),
	}

	items := make([]entity.LandItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, entity.LandItem{
			URIs:     item.GetUris(),
			Strategy: resolveStrategy(item.GetStrategy()),
		})
	}

	results, err := c.vcs.CheckMergeability(ctx, target, items)
	if err != nil {
		return nil, fmt.Errorf("check mergeability: %w", err)
	}

	allMergeable := true
	protoResults := make([]*pb.MergeabilityItemResult, 0, len(results))
	for _, r := range results {
		if !r.Mergeable {
			allMergeable = false
		}
		protoResults = append(protoResults, &pb.MergeabilityItemResult{
			Mergeable: r.Mergeable,
			Reason:    r.Reason,
		})
	}

	return &pb.CheckMergeabilityResponse{
		Mergeable: allMergeable,
		Results:   protoResults,
	}, nil
}

func resolveStrategy(s pb.Strategy) entity.LandStrategy {
	switch s {
	case pb.Strategy_STRATEGY_REBASE:
		return entity.LandStrategyRebase
	case pb.Strategy_STRATEGY_SQUASH_REBASE:
		return entity.LandStrategySquashRebase
	case pb.Strategy_STRATEGY_MERGE:
		return entity.LandStrategyMerge
	default:
		return entity.LandStrategyUnknown
	}
}

func outcomeStatusToProto(s vcs.OutcomeStatus) pb.OutcomeStatus {
	switch s {
	case vcs.OutcomeStatusCommitted:
		return pb.OutcomeStatus_OUTCOME_STATUS_COMMITTED
	case vcs.OutcomeStatusAlreadyExisted:
		return pb.OutcomeStatus_OUTCOME_STATUS_ALREADY_EXISTED
	default:
		return pb.OutcomeStatus_OUTCOME_STATUS_UNSPECIFIED
	}
}
