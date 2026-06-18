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

package mergeconflictcheck

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"github.com/uber/submitqueue/platform/base/change"
	entityqueue "github.com/uber/submitqueue/platform/base/messagequeue"
	"github.com/uber/submitqueue/platform/base/mergestrategy"
	"github.com/uber/submitqueue/platform/consumer"
	queuemock "github.com/uber/submitqueue/platform/extension/messagequeue/mock"
	"github.com/uber/submitqueue/runway/core/topickey"
	"github.com/uber/submitqueue/runway/entity"
	vcsmock "github.com/uber/submitqueue/runway/extension/vcs/mock"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
)

func testRequest() entity.MergeRequest {
	return entity.MergeRequest{
		ID:        "queue-a/42",
		QueueName: "queue-a",
		Steps: []entity.MergeStep{
			{
				StepID:   "queue-a/1",
				Changes:  []change.Change{{URIs: []string{"github://uber/repo/pull/1/abcdef0123456789abcdef0123456789abcdef01"}}},
				Strategy: mergestrategy.MergeStrategyRebase,
			},
		},
	}
}

func captureRegistry(t *testing.T, ctrl *gomock.Controller, publishErr error, captured *entityqueue.Message) consumer.TopicRegistry {
	t.Helper()

	mockPub := queuemock.NewMockPublisher(ctrl)
	mockPub.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, msg entityqueue.Message) error {
			if captured != nil {
				*captured = msg
			}
			return publishErr
		},
	).AnyTimes()

	mockQ := queuemock.NewMockQueue(ctrl)
	mockQ.EXPECT().Publisher().Return(mockPub).AnyTimes()

	registry, err := consumer.NewTopicRegistry([]consumer.TopicConfig{
		{Key: topickey.TopicKeyMergeConflictCheckSignal, Name: "merge-conflict-checker-signal", Queue: mockQ},
	})
	require.NoError(t, err)
	return registry
}

func makeDelivery(t *testing.T, ctrl *gomock.Controller, payload []byte) *queuemock.MockDelivery {
	t.Helper()
	msg := entityqueue.NewMessage("queue-a/42", payload, "queue-a", nil)
	delivery := queuemock.NewMockDelivery(ctrl)
	delivery.EXPECT().Message().Return(msg).AnyTimes()
	delivery.EXPECT().Attempt().Return(1).AnyTimes()
	return delivery
}

func newTestController(t *testing.T, ctrl *gomock.Controller, mockVCS *vcsmock.MockVCS, publishErr error, captured *entityqueue.Message) *Controller {
	t.Helper()

	factory := vcsmock.NewMockFactory(ctrl)
	factory.EXPECT().For(gomock.Any()).Return(mockVCS, nil).AnyTimes()

	return NewController(Params{
		Logger:        zaptest.NewLogger(t).Sugar(),
		Scope:         tally.NoopScope,
		Registry:      captureRegistry(t, ctrl, publishErr, captured),
		VCSFactory:    factory,
		TopicKey:      topickey.TopicKeyMergeConflictCheck,
		ConsumerGroup: "runway-merge-conflict-check",
	})
}

func TestNewController(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	controller := newTestController(t, ctrl, mockVCS, nil, nil)

	assert.Equal(t, "merge-conflict-check", controller.Name())
	assert.Equal(t, topickey.TopicKeyMergeConflictCheck, controller.TopicKey())
	assert.Equal(t, "runway-merge-conflict-check", controller.ConsumerGroup())
}

func TestProcess_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)

	expectedResult := entity.MergeResult{
		ID:      "queue-a/42",
		Success: true,
		Steps:   []entity.StepResult{{StepID: "queue-a/1"}},
	}
	mockVCS.EXPECT().CheckMergeability(gomock.Any(), gomock.Any()).Return(expectedResult, nil)

	var captured entityqueue.Message
	controller := newTestController(t, ctrl, mockVCS, nil, &captured)

	req := testRequest()
	payload, err := req.ToBytes()
	require.NoError(t, err)

	delivery := makeDelivery(t, ctrl, payload)
	require.NoError(t, controller.Process(context.Background(), delivery))

	assert.Equal(t, "queue-a/42", captured.ID)
	assert.Equal(t, "queue-a", captured.PartitionKey)

	var result entity.MergeResult
	result, err = entity.MergeResultFromBytes(captured.Payload)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "queue-a/42", result.ID)
}

func TestProcess_Errors(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "invalid json", payload: []byte(`not json`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockVCS := vcsmock.NewMockVCS(ctrl)
			controller := newTestController(t, ctrl, mockVCS, nil, nil)
			delivery := makeDelivery(t, ctrl, tt.payload)

			require.Error(t, controller.Process(context.Background(), delivery))
		})
	}
}

func TestProcess_VCSError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockVCS.EXPECT().CheckMergeability(gomock.Any(), gomock.Any()).Return(entity.MergeResult{}, fmt.Errorf("vcs unavailable"))

	controller := newTestController(t, ctrl, mockVCS, nil, nil)

	req := testRequest()
	payload, err := req.ToBytes()
	require.NoError(t, err)

	delivery := makeDelivery(t, ctrl, payload)
	require.Error(t, controller.Process(context.Background(), delivery))
}

func TestProcess_PublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockVCS := vcsmock.NewMockVCS(ctrl)
	mockVCS.EXPECT().CheckMergeability(gomock.Any(), gomock.Any()).Return(entity.MergeResult{ID: "queue-a/42", Success: true}, nil)

	controller := newTestController(t, ctrl, mockVCS, assert.AnError, nil)

	req := testRequest()
	payload, err := req.ToBytes()
	require.NoError(t, err)

	delivery := makeDelivery(t, ctrl, payload)
	require.Error(t, controller.Process(context.Background(), delivery))
}
