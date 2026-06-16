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

func TestCheck_SerializationRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		check Check
	}{
		{
			name: "single change single URI",
			check: Check{
				Queue:        "go-code-main",
				RequestID:    "go-code-main/42",
				Repo:         "uber/submitqueue",
				TargetBranch: "main",
				Changes:      []change.Change{{URIs: []string{"github://uber/submitqueue/pull/123/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}},
				Strategy:     mergestrategy.MergeStrategyRebase,
			},
		},
		{
			name: "multiple changes",
			check: Check{
				Queue:        "queue1",
				RequestID:    "queue1/100",
				Repo:         "uber/repo-a",
				TargetBranch: "main",
				Changes: []change.Change{
					{URIs: []string{"github://uber/repo-a/pull/101/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
					{URIs: []string{"github://uber/repo-a/pull/102/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
				},
				Strategy: mergestrategy.MergeStrategySquashRebase,
			},
		},
		{
			name: "stacked diff with multiple URIs",
			check: Check{
				Queue:        "queue2",
				RequestID:    "queue2/200",
				Repo:         "uber/submitqueue",
				TargetBranch: "release",
				Changes: []change.Change{{URIs: []string{
					"github://uber/submitqueue/pull/10/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"github://uber/submitqueue/pull/11/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				}}},
				Strategy: mergestrategy.MergeStrategyMerge,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.check.ToBytes()
			require.NoError(t, err)

			deserialized, err := CheckFromBytes(data)
			require.NoError(t, err)

			assert.Equal(t, tt.check, deserialized)
		})
	}
}

func TestCheckFromBytes_InvalidJSON(t *testing.T) {
	_, err := CheckFromBytes([]byte(`{"invalid": json"}`))
	assert.Error(t, err)
}

func TestCheckFromBytes_EmptyData(t *testing.T) {
	c, err := CheckFromBytes([]byte(`{}`))
	require.NoError(t, err)

	assert.Empty(t, c.Queue)
	assert.Empty(t, c.RequestID)
	assert.Equal(t, mergestrategy.MergeStrategyUnknown, c.Strategy)
}

func TestCheckResult_SerializationRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		result CheckResult
	}{
		{
			name: "all mergeable",
			result: CheckResult{
				Queue:     "go-code-main",
				RequestID: "go-code-main/42",
				Results: []MergeabilityResult{
					{Change: change.Change{URIs: []string{"github://uber/submitqueue/pull/1/aaaa"}}, Mergeable: true},
					{Change: change.Change{URIs: []string{"github://uber/submitqueue/pull/2/bbbb"}}, Mergeable: true},
				},
			},
		},
		{
			name: "some unmergeable",
			result: CheckResult{
				Queue:     "queue1",
				RequestID: "queue1/100",
				Results: []MergeabilityResult{
					{Change: change.Change{URIs: []string{"github://uber/repo/pull/1/aaaa"}}, Mergeable: true},
					{Change: change.Change{URIs: []string{"github://uber/repo/pull/2/bbbb"}}, Mergeable: false, Reason: "conflicts with src/main.go"},
				},
			},
		},
		{
			name: "empty results",
			result: CheckResult{
				Queue:     "queue2",
				RequestID: "queue2/200",
				Results:   []MergeabilityResult{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.result.ToBytes()
			require.NoError(t, err)

			deserialized, err := CheckResultFromBytes(data)
			require.NoError(t, err)

			assert.Equal(t, tt.result, deserialized)
		})
	}
}

func TestCheckResultFromBytes_InvalidJSON(t *testing.T) {
	_, err := CheckResultFromBytes([]byte(`not json`))
	assert.Error(t, err)
}
