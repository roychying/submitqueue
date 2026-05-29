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

// Package noop provides a build.BuildManager that performs no real work:
// every triggered build immediately succeeds. It is intended as a stub for
// wiring tests and as a best-case baseline where every build passes.
package noop

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/uber/submitqueue/entity"
	"github.com/uber/submitqueue/extension/build"
)

// manager is a build.BuildManager that does no real work and reports every
// build as immediately succeeded. The atomic counter hands out unique build
// IDs and makes the type safe for concurrent use.
type manager struct {
	counter atomic.Uint64
}

// New returns a build.BuildManager that performs no real work.
func New() build.BuildManager {
	return &manager{}
}

// Trigger returns a unique build ID and BuildStatusSucceeded without
// contacting any provider. Inputs are ignored.
func (m *manager) Trigger(_ context.Context, _ string, _ []entity.BuildChange) (string, entity.BuildStatus, error) {
	return fmt.Sprintf("noop-%d", m.counter.Add(1)), entity.BuildStatusSucceeded, nil
}

// Status always reports BuildStatusSucceeded with no metadata.
func (m *manager) Status(_ context.Context, _ string) (entity.BuildStatus, entity.BuildMetadata, error) {
	return entity.BuildStatusSucceeded, nil, nil
}

// Cancel is a no-op.
func (m *manager) Cancel(_ context.Context, _ string) error {
	return nil
}

// Close is a no-op.
func (m *manager) Close() error {
	return nil
}
