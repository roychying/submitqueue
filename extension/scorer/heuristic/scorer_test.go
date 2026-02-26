package heuristic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/extension/scorer"
)

func TestScorer_Score(t *testing.T) {
	tests := []struct {
		name     string
		buckets  []Bucket
		statFunc StatFunc
		stats    scorer.ChangeStats
		want     float64
		wantErr  bool
	}{
		{
			name: "single bucket covering all values",
			buckets: []Bucket{
				{Min: 0, Max: 1000, Score: 0.9},
			},
			statFunc: FilesChanged,
			stats:    scorer.ChangeStats{FilesChanged: 5},
			want:     0.9,
		},
		{
			name: "multiple buckets with different ranges",
			buckets: []Bucket{
				{Min: 0, Max: 5, Score: 0.95},
				{Min: 6, Max: 20, Score: 0.75},
				{Min: 21, Max: 100, Score: 0.5},
			},
			statFunc: FilesChanged,
			stats:    scorer.ChangeStats{FilesChanged: 10},
			want:     0.75,
		},
		{
			name: "exact min boundary",
			buckets: []Bucket{
				{Min: 0, Max: 5, Score: 0.95},
				{Min: 6, Max: 20, Score: 0.75},
			},
			statFunc: FilesChanged,
			stats:    scorer.ChangeStats{FilesChanged: 6},
			want:     0.75,
		},
		{
			name: "exact max boundary",
			buckets: []Bucket{
				{Min: 0, Max: 5, Score: 0.95},
				{Min: 6, Max: 20, Score: 0.75},
			},
			statFunc: FilesChanged,
			stats:    scorer.ChangeStats{FilesChanged: 5},
			want:     0.95,
		},
		{
			name: "no matching bucket",
			buckets: []Bucket{
				{Min: 0, Max: 5, Score: 0.95},
				{Min: 10, Max: 20, Score: 0.75},
			},
			statFunc: FilesChanged,
			stats:    scorer.ChangeStats{FilesChanged: 7},
			wantErr:  true,
		},
		{
			name: "lines added stat function",
			buckets: []Bucket{
				{Min: 0, Max: 100, Score: 0.9},
				{Min: 101, Max: 500, Score: 0.7},
			},
			statFunc: LinesAdded,
			stats:    scorer.ChangeStats{LinesAdded: 200},
			want:     0.7,
		},
		{
			name: "lines deleted stat function",
			buckets: []Bucket{
				{Min: 0, Max: 50, Score: 0.95},
				{Min: 51, Max: 200, Score: 0.8},
			},
			statFunc: LinesDeleted,
			stats:    scorer.ChangeStats{LinesDeleted: 10},
			want:     0.95,
		},
		{
			name: "zero-value change stats",
			buckets: []Bucket{
				{Min: 0, Max: 0, Score: 1.0},
				{Min: 1, Max: 100, Score: 0.8},
			},
			statFunc: FilesChanged,
			stats:    scorer.ChangeStats{},
			want:     1.0,
		},
		{
			name: "first matching bucket wins",
			buckets: []Bucket{
				{Min: 0, Max: 10, Score: 0.9},
				{Min: 5, Max: 15, Score: 0.7},
			},
			statFunc: FilesChanged,
			stats:    scorer.ChangeStats{FilesChanged: 7},
			want:     0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.buckets, tt.statFunc)
			require.NoError(t, err)
			got, err := s.Score(context.Background(), tt.stats)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNew_NilStatFunc(t *testing.T) {
	_, err := New([]Bucket{{Min: 0, Max: 10, Score: 0.85}}, nil)
	require.Error(t, err)
}
