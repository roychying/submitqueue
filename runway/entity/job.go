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

// ResultStatus defines the possible outcomes of a landing job.
type ResultStatus string

const (
	// ResultStatusUnknown is the unreachable zero value, set by default when
	// the structure is initialized. It should never be seen in the system.
	ResultStatusUnknown ResultStatus = ""
	// ResultStatusSucceeded means all items in the job were landed successfully.
	ResultStatusSucceeded ResultStatus = "succeeded"
	// ResultStatusConflict means pre-validation detected a merge conflict.
	ResultStatusConflict ResultStatus = "conflict"
	// ResultStatusError means an infrastructure failure prevented landing.
	ResultStatusError ResultStatus = "error"
)

// Job is the inbound message on the runway-land topic. SubmitQueue publishes
// one Job per batch to land a scored batch. The job carries the resolved content
// for each request in the batch — runway has no access to SubmitQueue's request
// store, so the message must be self-contained.
type Job struct {
	// ID is the unique job identifier (idempotency key).
	ID string `json:"id"`
	// BatchID is the SubmitQueue batch ID for correlation.
	BatchID string `json:"batch_id"`
	// Queue is the SubmitQueue queue name.
	Queue string `json:"queue"`
	// Repo identifies the repository (e.g., "uber/submitqueue").
	Repo string `json:"repo"`
	// TargetBranch is the destination branch (e.g., "main").
	TargetBranch string `json:"target_branch"`
	// Items is the per-request content, in landing order.
	Items []JobItem `json:"items"`
}

// ToBytes serializes the Job to JSON bytes for queue message payload.
func (j Job) ToBytes() ([]byte, error) {
	return json.Marshal(j)
}

// JobFromBytes deserializes a Job from JSON bytes.
func JobFromBytes(data []byte) (Job, error) {
	var j Job
	err := json.Unmarshal(data, &j)
	return j, err
}

// JobItem carries one request's resolved content within a Job.
type JobItem struct {
	// RequestID is the SubmitQueue request ID for correlation.
	RequestID string `json:"request_id"`
	// Change is the code change to land.
	Change change.Change `json:"change"`
	// Strategy is the per-request landing strategy.
	Strategy mergestrategy.MergeStrategy `json:"strategy"`
}

// Outcome describes what happened to a single item within a successful landing.
type Outcome struct {
	// RequestID is the SubmitQueue request ID for correlation.
	RequestID string `json:"request_id"`
	// Change is the input change this outcome corresponds to.
	Change change.Change `json:"change"`
	// CommitSHAs lists the commits produced on the target branch, in apply order.
	// Empty when AlreadyExisted is true.
	CommitSHAs []string `json:"commit_shas"`
	// AlreadyExisted is true if the change was already present on the target
	// branch (no new commits were produced).
	AlreadyExisted bool `json:"already_existed"`
}

// Result is the outbound message published to the sq-land-result topic.
// It carries the landing outcome back to SubmitQueue.
type Result struct {
	// JobID correlates to Job.ID.
	JobID string `json:"job_id"`
	// BatchID is the SubmitQueue batch ID.
	BatchID string `json:"batch_id"`
	// Queue is the SubmitQueue queue name (partition key for the outbound topic).
	Queue string `json:"queue"`
	// Status is the overall landing outcome.
	Status ResultStatus `json:"status"`
	// Outcomes is one entry per item, in landing order. Populated when Status
	// is ResultStatusSucceeded.
	Outcomes []Outcome `json:"outcomes,omitempty"`
	// Error is a human-readable description of the failure. Populated when
	// Status is ResultStatusConflict or ResultStatusError.
	Error string `json:"error,omitempty"`
}

// ToBytes serializes the Result to JSON bytes for queue message payload.
func (r Result) ToBytes() ([]byte, error) {
	return json.Marshal(r)
}

// ResultFromBytes deserializes a Result from JSON bytes.
func ResultFromBytes(data []byte) (Result, error) {
	var r Result
	err := json.Unmarshal(data, &r)
	return r, err
}
