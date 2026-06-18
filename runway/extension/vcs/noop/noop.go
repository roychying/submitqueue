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

// Package noop provides a no-op VCS implementation for local development and
// testing. CheckMergeability always reports success; Land produces synthetic
// output IDs from an atomic counter.
package noop

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/uber/submitqueue/runway/entity"
	"github.com/uber/submitqueue/runway/extension/vcs"
)

var _ vcs.VCS = (*VCS)(nil)

// VCS is a no-op implementation that always succeeds.
type VCS struct {
	seq atomic.Uint64
}

// New returns a new no-op VCS instance.
func New() *VCS { return &VCS{} }

func (v *VCS) CheckMergeability(_ context.Context, req entity.MergeRequest) (entity.MergeResult, error) {
	steps := make([]entity.StepResult, len(req.Steps))
	for i, s := range req.Steps {
		steps[i] = entity.StepResult{StepID: s.StepID}
	}
	return entity.MergeResult{
		ID:      req.ID,
		Success: true,
		Steps:   steps,
	}, nil
}

func (v *VCS) Land(_ context.Context, req entity.MergeRequest) (entity.MergeResult, error) {
	steps := make([]entity.StepResult, len(req.Steps))
	for i, s := range req.Steps {
		n := v.seq.Add(1)
		steps[i] = entity.StepResult{
			StepID:    s.StepID,
			OutputIDs: []string{fmt.Sprintf("%040x", n)},
		}
	}
	return entity.MergeResult{
		ID:      req.ID,
		Success: true,
		Steps:   steps,
	}, nil
}
