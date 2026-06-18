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

	"github.com/uber-go/tally"
	entityqueue "github.com/uber/submitqueue/platform/base/messagequeue"
	"github.com/uber/submitqueue/platform/consumer"
	"github.com/uber/submitqueue/platform/metrics"
	"github.com/uber/submitqueue/runway/core/topickey"
	"github.com/uber/submitqueue/runway/entity"
	"github.com/uber/submitqueue/runway/extension/vcs"
	"go.uber.org/zap"
)

var _ consumer.Controller = (*Controller)(nil)

// Controller handles merge-conflict-check queue messages. It performs a dry-run
// merge and publishes per-step mergeability results back to the signal queue.
type Controller struct {
	logger        *zap.SugaredLogger
	metricsScope  tally.Scope
	registry      consumer.TopicRegistry
	vcsFactory    vcs.Factory
	topicKey      consumer.TopicKey
	consumerGroup string
}

// Params are the parameters for creating a new merge-conflict-check controller.
type Params struct {
	Registry      consumer.TopicRegistry
	VCSFactory    vcs.Factory
	TopicKey      consumer.TopicKey
	ConsumerGroup string

	Scope  tally.Scope
	Logger *zap.SugaredLogger
}

// NewController creates a new merge-conflict-check controller.
func NewController(p Params) *Controller {
	return &Controller{
		logger:        p.Logger.Named("merge_conflict_check_controller"),
		metricsScope:  p.Scope.SubScope("merge_conflict_check_controller"),
		registry:      p.Registry,
		vcsFactory:    p.VCSFactory,
		topicKey:      p.TopicKey,
		consumerGroup: p.ConsumerGroup,
	}
}

func (c *Controller) Process(ctx context.Context, delivery consumer.Delivery) (retErr error) {
	const opName = "process"

	op := metrics.Begin(c.metricsScope, opName)
	defer func() { op.Complete(retErr) }()

	msg := delivery.Message()

	req, err := entity.MergeRequestFromBytes(msg.Payload)
	if err != nil {
		metrics.NamedCounter(c.metricsScope, opName, "deserialize_errors", 1)
		return fmt.Errorf("failed to deserialize merge request: %w", err)
	}

	c.logger.Infow("received merge-conflict-check request",
		"request_id", req.ID,
		"queue", req.QueueName,
		"step_count", len(req.Steps),
		"attempt", delivery.Attempt(),
	)

	v, err := c.vcsFactory.For(vcs.Config{QueueName: req.QueueName})
	if err != nil {
		metrics.NamedCounter(c.metricsScope, opName, "factory_errors", 1)
		return fmt.Errorf("failed to build VCS for queue %s: %w", req.QueueName, err)
	}

	result, err := v.CheckMergeability(ctx, req)
	if err != nil {
		metrics.NamedCounter(c.metricsScope, opName, "check_errors", 1)
		return fmt.Errorf("merge-conflict check failed for request %s: %w", req.ID, err)
	}

	if err := c.publishResult(ctx, result, req.QueueName); err != nil {
		metrics.NamedCounter(c.metricsScope, opName, "publish_errors", 1)
		return fmt.Errorf("failed to publish check result for request %s: %w", req.ID, err)
	}

	c.logger.Infow("published merge-conflict-check result",
		"request_id", req.ID,
		"success", result.Success,
	)

	return nil
}

func (c *Controller) publishResult(ctx context.Context, result entity.MergeResult, partitionKey string) error {
	payload, err := result.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to serialize merge result: %w", err)
	}

	q, ok := c.registry.Queue(topickey.TopicKeyMergeConflictCheckSignal)
	if !ok {
		return fmt.Errorf("no queue registered for topic key %s", topickey.TopicKeyMergeConflictCheckSignal)
	}

	topicName, ok := c.registry.TopicName(topickey.TopicKeyMergeConflictCheckSignal)
	if !ok {
		return fmt.Errorf("no topic name registered for topic key %s", topickey.TopicKeyMergeConflictCheckSignal)
	}

	msg := entityqueue.NewMessage(result.ID, payload, partitionKey, nil)
	if err := q.Publisher().Publish(ctx, topicName, msg); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (c *Controller) Name() string                    { return "merge-conflict-check" }
func (c *Controller) TopicKey() consumer.TopicKey      { return c.topicKey }
func (c *Controller) ConsumerGroup() string            { return c.consumerGroup }
