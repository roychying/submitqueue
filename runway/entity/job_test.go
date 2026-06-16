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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/entity/change"
	"github.com/uber/submitqueue/entity/mergestrategy"
)

func TestJob_SerializationRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		job  Job
	}{
		{
			name: "single item rebase",
			job: Job{
				ID:           "job-001",
				BatchID:      "go-code-main/batch/1",
				Queue:        "go-code-main",
				Repo:         "uber/submitqueue",
				TargetBranch: "main",
				Items: []JobItem{
					{
						RequestID: "go-code-main/42",
						Change:    change.Change{URIs: []string{"github://uber/submitqueue/pull/123/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
						Strategy:  mergestrategy.MergeStrategyRebase,
					},
				},
			},
		},
		{
			name: "multiple items mixed strategies",
			job: Job{
				ID:           "job-002",
				BatchID:      "queue1/batch/5",
				Queue:        "queue1",
				Repo:         "uber/repo-a",
				TargetBranch: "main",
				Items: []JobItem{
					{
						RequestID: "queue1/10",
						Change:    change.Change{URIs: []string{"github://uber/repo-a/pull/10/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
						Strategy:  mergestrategy.MergeStrategyRebase,
					},
					{
						RequestID: "queue1/11",
						Change:    change.Change{URIs: []string{"github://uber/repo-a/pull/11/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
						Strategy:  mergestrategy.MergeStrategySquashRebase,
					},
					{
						RequestID: "queue1/12",
						Change:    change.Change{URIs: []string{"github://uber/repo-a/pull/12/cccccccccccccccccccccccccccccccccccccccc"}},
						Strategy:  mergestrategy.MergeStrategyMerge,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.job.ToBytes()
			require.NoError(t, err)

			deserialized, err := JobFromBytes(data)
			require.NoError(t, err)

			assert.Equal(t, tt.job, deserialized)
		})
	}
}

func TestJobFromBytes_InvalidJSON(t *testing.T) {
	_, err := JobFromBytes([]byte(`{"invalid": json"}`))
	assert.Error(t, err)
}

func TestJobFromBytes_EmptyData(t *testing.T) {
	j, err := JobFromBytes([]byte(`{}`))
	require.NoError(t, err)

	assert.Empty(t, j.ID)
	assert.Empty(t, j.BatchID)
	assert.Empty(t, j.Queue)
	assert.Nil(t, j.Items)
}

func TestResult_SerializationRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		result Result
	}{
		{
			name: "succeeded with outcomes",
			result: Result{
				JobID:   "job-001",
				BatchID: "go-code-main/batch/1",
				Queue:   "go-code-main",
				Status:  ResultStatusSucceeded,
				Outcomes: []Outcome{
					{
						RequestID:  "go-code-main/42",
						Change:     change.Change{URIs: []string{"github://uber/submitqueue/pull/123/aaaa"}},
						CommitSHAs: []string{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
					},
					{
						RequestID:      "go-code-main/43",
						Change:         change.Change{URIs: []string{"github://uber/submitqueue/pull/124/bbbb"}},
						AlreadyExisted: true,
					},
				},
			},
		},
		{
			name: "conflict",
			result: Result{
				JobID:   "job-002",
				BatchID: "queue1/batch/5",
				Queue:   "queue1",
				Status:  ResultStatusConflict,
				Error:   "item queue1/11 conflicts with src/main.go",
			},
		},
		{
			name: "error",
			result: Result{
				JobID:   "job-003",
				BatchID: "queue2/batch/10",
				Queue:   "queue2",
				Status:  ResultStatusError,
				Error:   "git push failed: remote rejected",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.result.ToBytes()
			require.NoError(t, err)

			deserialized, err := ResultFromBytes(data)
			require.NoError(t, err)

			assert.Equal(t, tt.result, deserialized)
		})
	}
}

func TestResultFromBytes_InvalidJSON(t *testing.T) {
	_, err := ResultFromBytes([]byte(`not json`))
	assert.Error(t, err)
}

func TestResultStatus_Values(t *testing.T) {
	assert.Equal(t, ResultStatus(""), ResultStatusUnknown)
	assert.Equal(t, ResultStatus("succeeded"), ResultStatusSucceeded)
	assert.Equal(t, ResultStatus("conflict"), ResultStatusConflict)
	assert.Equal(t, ResultStatus("error"), ResultStatusError)
}
