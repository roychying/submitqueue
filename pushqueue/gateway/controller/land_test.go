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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally/v4"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/uber/submitqueue/core/errs"
	"github.com/uber/submitqueue/pushqueue/entity"
	landqueuemock "github.com/uber/submitqueue/pushqueue/extension/landqueue/mock"
	"github.com/uber/submitqueue/pushqueue/extension/vcs"
	vcsmock "github.com/uber/submitqueue/pushqueue/extension/vcs/mock"
	pb "github.com/uber/submitqueue/pushqueue/gateway/protopb"
)

func newTestLandController(t *testing.T, v vcs.VCS, q *landqueuemock.MockQueue) *LandController {
	return NewLandController(zaptest.NewLogger(t).Sugar(), tally.NoopScope, v, q)
}

func testQueue() *pb.Queue {
	return &pb.Queue{
		Name:    "test-queue",
		Address: "git@github.com:uber/repo.git",
		Target:  "main",
	}
}

func testTarget() entity.QueueTarget {
	return entity.QueueTarget{
		Name:    "test-queue",
		Address: "git@github.com:uber/repo.git",
		Target:  "main",
	}
}

func TestNewLandController(t *testing.T) {
	ctrl := gomock.NewController(t)
	c := newTestLandController(t, vcsmock.NewMockVCS(ctrl), landqueuemock.NewMockQueue(ctrl))
	require.NotNil(t, c)
}

func TestLand_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	target := testTarget()
	items := []entity.LandItem{{
		URIs:     []string{"github://uber/repo/pull/1/abc123"},
		Strategy: entity.LandStrategyRebase,
	}}

	gomock.InOrder(
		mockQueue.EXPECT().Enqueue(gomock.Any(), target, items).Return(nil),
		mockQueue.EXPECT().Wait(gomock.Any(), target).Return(nil),
		mockVCS.EXPECT().Push(gomock.Any(), target, items).Return(vcs.PushResult{
			Outcomes: []vcs.ItemOutcome{{
				Status:      vcs.OutcomeStatusCommitted,
				RevisionIDs: []string{"deadbeef"},
			}},
		}, nil),
		mockVCS.EXPECT().Finalize(gomock.Any(), target, items).Return(nil),
		mockQueue.EXPECT().Dequeue(gomock.Any(), target).Return(nil),
	)

	c := newTestLandController(t, mockVCS, mockQueue)
	resp, err := c.Land(context.Background(), &pb.LandRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{
			Uris:     []string{"github://uber/repo/pull/1/abc123"},
			Strategy: pb.Strategy_STRATEGY_REBASE,
		}},
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	require.Len(t, resp.Outcomes, 1)
	assert.Equal(t, pb.OutcomeStatus_OUTCOME_STATUS_COMMITTED, resp.Outcomes[0].Status)
	assert.Equal(t, []string{"deadbeef"}, resp.Outcomes[0].RevisionIds)
	assert.Empty(t, resp.FinalizeError)
}

func TestLand_FinalizeErrorRecordedButNotFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	target := testTarget()

	mockQueue.EXPECT().Enqueue(gomock.Any(), target, gomock.Any()).Return(nil)
	mockQueue.EXPECT().Wait(gomock.Any(), target).Return(nil)
	mockVCS.EXPECT().Push(gomock.Any(), target, gomock.Any()).Return(vcs.PushResult{
		Outcomes: []vcs.ItemOutcome{{Status: vcs.OutcomeStatusCommitted, RevisionIDs: []string{"abc"}}},
	}, nil)
	mockVCS.EXPECT().Finalize(gomock.Any(), target, gomock.Any()).Return(fmt.Errorf("GitHub API down"))
	mockQueue.EXPECT().Dequeue(gomock.Any(), target).Return(nil)

	c := newTestLandController(t, mockVCS, mockQueue)
	resp, err := c.Land(context.Background(), &pb.LandRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{Uris: []string{"github://uber/repo/pull/1/abc"}}},
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "GitHub API down", resp.FinalizeError)
}

func TestLand_PushConflictReturnsUserError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	target := testTarget()

	mockQueue.EXPECT().Enqueue(gomock.Any(), target, gomock.Any()).Return(nil)
	mockQueue.EXPECT().Wait(gomock.Any(), target).Return(nil)
	mockVCS.EXPECT().Push(gomock.Any(), target, gomock.Any()).Return(vcs.PushResult{}, fmt.Errorf("apply: %w", vcs.ErrConflict))
	mockQueue.EXPECT().Dequeue(gomock.Any(), target).Return(nil)

	c := newTestLandController(t, mockVCS, mockQueue)
	_, err := c.Land(context.Background(), &pb.LandRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{Uris: []string{"github://uber/repo/pull/1/abc"}}},
	})

	require.Error(t, err)
	assert.True(t, errs.IsUserError(err))
	assert.False(t, errs.IsRetryable(err))
}

func TestLand_StaleHeadReturnsRetryableError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	target := testTarget()

	mockQueue.EXPECT().Enqueue(gomock.Any(), target, gomock.Any()).Return(nil)
	mockQueue.EXPECT().Wait(gomock.Any(), target).Return(nil)
	mockVCS.EXPECT().Push(gomock.Any(), target, gomock.Any()).Return(vcs.PushResult{}, vcs.ErrStaleHead)
	mockQueue.EXPECT().Dequeue(gomock.Any(), target).Return(nil)

	c := newTestLandController(t, mockVCS, mockQueue)
	_, err := c.Land(context.Background(), &pb.LandRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{Uris: []string{"github://uber/repo/pull/1/abc"}}},
	})

	require.Error(t, err)
	assert.True(t, errs.IsRetryable(err))
}

func TestLand_InfraErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	target := testTarget()

	mockQueue.EXPECT().Enqueue(gomock.Any(), target, gomock.Any()).Return(nil)
	mockQueue.EXPECT().Wait(gomock.Any(), target).Return(nil)
	mockVCS.EXPECT().Push(gomock.Any(), target, gomock.Any()).Return(vcs.PushResult{}, fmt.Errorf("ssh: connection refused"))
	mockQueue.EXPECT().Dequeue(gomock.Any(), target).Return(nil)

	c := newTestLandController(t, mockVCS, mockQueue)
	_, err := c.Land(context.Background(), &pb.LandRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{Uris: []string{"github://uber/repo/pull/1/abc"}}},
	})

	require.Error(t, err)
	assert.False(t, errs.IsUserError(err))
	assert.False(t, errs.IsRetryable(err))
}

func TestLand_EnqueueFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	mockQueue.EXPECT().Enqueue(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("queue full"))

	c := newTestLandController(t, mockVCS, mockQueue)
	_, err := c.Land(context.Background(), &pb.LandRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{Uris: []string{"github://uber/repo/pull/1/abc"}}},
	})

	require.Error(t, err)
}

func TestLand_WaitFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	mockQueue.EXPECT().Enqueue(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockQueue.EXPECT().Wait(gomock.Any(), gomock.Any()).Return(fmt.Errorf("context cancelled"))
	mockQueue.EXPECT().Dequeue(gomock.Any(), gomock.Any()).Return(nil)

	c := newTestLandController(t, mockVCS, mockQueue)
	_, err := c.Land(context.Background(), &pb.LandRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{Uris: []string{"github://uber/repo/pull/1/abc"}}},
	})

	require.Error(t, err)
}

func TestLand_StrategyMapping(t *testing.T) {
	testCases := []struct {
		name     string
		proto    pb.Strategy
		expected entity.LandStrategy
	}{
		{"rebase", pb.Strategy_STRATEGY_REBASE, entity.LandStrategyRebase},
		{"squash_rebase", pb.Strategy_STRATEGY_SQUASH_REBASE, entity.LandStrategySquashRebase},
		{"merge", pb.Strategy_STRATEGY_MERGE, entity.LandStrategyMerge},
		{"unspecified", pb.Strategy_STRATEGY_UNSPECIFIED, entity.LandStrategyUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, resolveStrategy(tc.proto))
		})
	}
}

func TestCheckMergeability_AllMergeable(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	target := testTarget()

	mockVCS.EXPECT().CheckMergeability(gomock.Any(), target, gomock.Any()).Return([]vcs.MergeabilityResult{
		{Mergeable: true},
		{Mergeable: true},
	}, nil)

	c := newTestLandController(t, mockVCS, mockQueue)
	resp, err := c.CheckMergeability(context.Background(), &pb.CheckMergeabilityRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{
			{Uris: []string{"github://uber/repo/pull/1/abc"}},
			{Uris: []string{"github://uber/repo/pull/2/def"}},
		},
	})

	require.NoError(t, err)
	assert.True(t, resp.Mergeable)
	require.Len(t, resp.Results, 2)
	assert.True(t, resp.Results[0].Mergeable)
	assert.True(t, resp.Results[1].Mergeable)
}

func TestCheckMergeability_SomeNotMergeable(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	target := testTarget()

	mockVCS.EXPECT().CheckMergeability(gomock.Any(), target, gomock.Any()).Return([]vcs.MergeabilityResult{
		{Mergeable: true},
		{Mergeable: false, Reason: "conflicts with target"},
	}, nil)

	c := newTestLandController(t, mockVCS, mockQueue)
	resp, err := c.CheckMergeability(context.Background(), &pb.CheckMergeabilityRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{
			{Uris: []string{"github://uber/repo/pull/1/abc"}},
			{Uris: []string{"github://uber/repo/pull/2/def"}},
		},
	})

	require.NoError(t, err)
	assert.False(t, resp.Mergeable)
	require.Len(t, resp.Results, 2)
	assert.True(t, resp.Results[0].Mergeable)
	assert.False(t, resp.Results[1].Mergeable)
	assert.Equal(t, "conflicts with target", resp.Results[1].Reason)
}

func TestCheckMergeability_VCSError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockQueue := landqueuemock.NewMockQueue(ctrl)

	mockVCS.EXPECT().CheckMergeability(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("VCS unavailable"))

	c := newTestLandController(t, mockVCS, mockQueue)
	_, err := c.CheckMergeability(context.Background(), &pb.CheckMergeabilityRequest{
		Queue: testQueue(),
		Items: []*pb.LandItem{{Uris: []string{"github://uber/repo/pull/1/abc"}}},
	})

	require.Error(t, err)
}
