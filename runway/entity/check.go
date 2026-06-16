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

package entity

import (
	"encoding/json"

	"github.com/uber/submitqueue/entity/change"
	"github.com/uber/submitqueue/entity/mergestrategy"
)

// Check is the inbound message on the runway-check topic. SubmitQueue publishes
// one Check per request to determine whether the request's changes can merge
// cleanly against the target branch. The check is read-only — it does not
// mutate the target branch or any external state.
type Check struct {
	// Queue is the SubmitQueue queue name.
	Queue string `json:"queue"`
	// RequestID is the SubmitQueue request ID. Serves as the idempotency key.
	RequestID string `json:"request_id"`
	// Repo identifies the repository (e.g., "uber/submitqueue").
	Repo string `json:"repo"`
	// TargetBranch is the destination branch (e.g., "main").
	TargetBranch string `json:"target_branch"`
	// Changes is the set of code changes to check for mergeability.
	Changes []change.Change `json:"changes"`
	// Strategy is the landing strategy that would be used to land these changes.
	Strategy mergestrategy.MergeStrategy `json:"strategy"`
}

// ToBytes serializes the Check to JSON bytes for queue message payload.
func (c Check) ToBytes() ([]byte, error) {
	return json.Marshal(c)
}

// CheckFromBytes deserializes a Check from JSON bytes.
func CheckFromBytes(data []byte) (Check, error) {
	var c Check
	err := json.Unmarshal(data, &c)
	return c, err
}

// MergeabilityResult describes whether a single change can be applied cleanly
// to the target branch.
type MergeabilityResult struct {
	// Change is the input change this result corresponds to.
	Change change.Change `json:"change"`
	// Mergeable is true if the change can be applied cleanly.
	Mergeable bool `json:"mergeable"`
	// Reason is a human-readable explanation when Mergeable is false; empty when true.
	Reason string `json:"reason,omitempty"`
}

// CheckResult is the outbound message published to the sq-check-result topic.
// It carries per-change mergeability detail back to SubmitQueue.
type CheckResult struct {
	// Queue is the SubmitQueue queue name (partition key for the outbound topic).
	Queue string `json:"queue"`
	// RequestID correlates to Check.RequestID.
	RequestID string `json:"request_id"`
	// Results is one entry per change in the input Check.Changes.
	Results []MergeabilityResult `json:"results"`
}

// ToBytes serializes the CheckResult to JSON bytes for queue message payload.
func (r CheckResult) ToBytes() ([]byte, error) {
	return json.Marshal(r)
}

// CheckResultFromBytes deserializes a CheckResult from JSON bytes.
func CheckResultFromBytes(data []byte) (CheckResult, error) {
	var r CheckResult
	err := json.Unmarshal(data, &r)
	return r, err
}
