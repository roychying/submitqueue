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
	"github.com/uber/submitqueue/entity"
	"github.com/uber/submitqueue/extension/build"
)

func TestNew_ImplementsInterface(t *testing.T) {
	var _ build.BuildManager = New()
}

func TestManager_Trigger(t *testing.T) {
	m := New()
	ctx := context.Background()

	id1, status, err := m.Trigger(ctx, "queueA", []entity.BuildChange{
		{Action: entity.ChangeActionValidate},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, id1)
	assert.Equal(t, entity.BuildStatusSucceeded, status)

	// IDs are unique across calls, even with no changes.
	id2, _, err := m.Trigger(ctx, "queueA", nil)
	require.NoError(t, err)
	assert.NotEqual(t, id1, id2)
}

func TestManager_Status(t *testing.T) {
	m := New()

	status, meta, err := m.Status(context.Background(), "any-id")
	require.NoError(t, err)
	assert.Equal(t, entity.BuildStatusSucceeded, status)
	assert.Empty(t, meta)
}

func TestManager_Cancel(t *testing.T) {
	m := New()
	assert.NoError(t, m.Cancel(context.Background(), "any-id"))
}

func TestManager_Close(t *testing.T) {
	m := New()
	assert.NoError(t, m.Close())
}
