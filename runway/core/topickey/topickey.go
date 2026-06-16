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

// Package topickey defines Runway pipeline stage identifiers.
package topickey

import "github.com/uber/submitqueue/core/consumer"

// TopicKey is the shared pipeline stage identifier type.
type TopicKey = consumer.TopicKey

const (
	// TopicKeyCheck is the inbound topic where mergeability check requests arrive from SubmitQueue.
	TopicKeyCheck TopicKey = "check"
	// TopicKeyLand is the inbound topic where batch land jobs arrive from SubmitQueue.
	TopicKeyLand TopicKey = "land"
	// TopicKeyCheckResult is the outbound topic where check results are published back to SubmitQueue.
	TopicKeyCheckResult TopicKey = "checkresult"
	// TopicKeyLandResult is the outbound topic where land results are published back to SubmitQueue.
	TopicKeyLandResult TopicKey = "landresult"
)
