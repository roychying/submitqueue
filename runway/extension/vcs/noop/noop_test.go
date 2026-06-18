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

package noop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/platform/base/change"
	"github.com/uber/submitqueue/platform/base/mergestrategy"
	"github.com/uber/submitqueue/runway/entity"
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
			{
				StepID:   "queue-a/2",
				Changes:  []change.Change{{URIs: []string{"github://uber/repo/pull/2/89abcdef0123456789abcdef0123456789abcdef"}}},
				Strategy: mergestrategy.MergeStrategyMerge,
			},
		},
	}
}

func TestCheckMergeability(t *testing.T) {
	v := New()
	req := testRequest()

	res, err := v.CheckMergeability(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, req.ID, res.ID)
	assert.True(t, res.Success)
	require.Len(t, res.Steps, 2)
	assert.Equal(t, "queue-a/1", res.Steps[0].StepID)
	assert.Empty(t, res.Steps[0].OutputIDs)
	assert.Equal(t, "queue-a/2", res.Steps[1].StepID)
	assert.Empty(t, res.Steps[1].OutputIDs)
}

func TestLand(t *testing.T) {
	v := New()
	req := testRequest()

	res, err := v.Land(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, req.ID, res.ID)
	assert.True(t, res.Success)
	require.Len(t, res.Steps, 2)
	assert.Equal(t, "queue-a/1", res.Steps[0].StepID)
	require.Len(t, res.Steps[0].OutputIDs, 1)
	assert.NotEmpty(t, res.Steps[0].OutputIDs[0])
	assert.Equal(t, "queue-a/2", res.Steps[1].StepID)
	require.Len(t, res.Steps[1].OutputIDs, 1)
	assert.NotEmpty(t, res.Steps[1].OutputIDs[0])
}

func TestLand_UniqueOutputIDs(t *testing.T) {
	v := New()
	req := testRequest()

	res1, err := v.Land(context.Background(), req)
	require.NoError(t, err)
	res2, err := v.Land(context.Background(), req)
	require.NoError(t, err)

	assert.NotEqual(t, res1.Steps[0].OutputIDs[0], res2.Steps[0].OutputIDs[0])
}
