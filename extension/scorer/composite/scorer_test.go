package composite

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/extension/scorer"
	"github.com/uber/submitqueue/extension/scorer/heuristic"
)

func TestScorer_Score(t *testing.T) {
	fileScorer, err := heuristic.New([]heuristic.Bucket{
		{Min: 0, Max: 5, Score: 0.9},
		{Min: 6, Max: 50, Score: 0.7},
		{Min: 51, Max: 1000, Score: 0.4},
	}, heuristic.FilesChanged)
	require.NoError(t, err)

	depScorer, err := heuristic.New([]heuristic.Bucket{
		{Min: 0, Max: 10, Score: 0.95},
		{Min: 11, Max: 100, Score: 0.6},
		{Min: 101, Max: 10000, Score: 0.3},
	}, heuristic.DependencyCount)
	require.NoError(t, err)

	linesScorer, err := heuristic.New([]heuristic.Bucket{
		{Min: 0, Max: 100, Score: 0.85},
		{Min: 101, Max: 500, Score: 0.5},
	}, heuristic.LinesAdded)
	require.NoError(t, err)

	tests := []struct {
		name    string
		scorers []scorer.Scorer
		reduce  ReduceFunc
		stats   scorer.ChangeStats
		want    float64
		wantErr bool
	}{
		{
			name:    "min of two scorers",
			scorers: []scorer.Scorer{fileScorer, depScorer},
			reduce:  Min,
			stats:   scorer.ChangeStats{FilesChanged: 3, DependencyCount: 50},
			want:    0.6,
		},
		{
			name:    "max of two scorers",
			scorers: []scorer.Scorer{fileScorer, depScorer},
			reduce:  Max,
			stats:   scorer.ChangeStats{FilesChanged: 3, DependencyCount: 50},
			want:    0.9,
		},
		{
			name:    "avg of two scorers",
			scorers: []scorer.Scorer{fileScorer, depScorer},
			reduce:  Avg,
			stats:   scorer.ChangeStats{FilesChanged: 3, DependencyCount: 50},
			want:    0.75,
		},
		{
			name:    "single scorer passthrough",
			scorers: []scorer.Scorer{fileScorer},
			reduce:  Avg,
			stats:   scorer.ChangeStats{FilesChanged: 3},
			want:    0.9,
		},
		{
			name:    "child scorer error propagates",
			scorers: []scorer.Scorer{fileScorer, depScorer},
			reduce:  Min,
			stats:   scorer.ChangeStats{FilesChanged: 3, DependencyCount: -1},
			wantErr: true,
		},
		{
			name:    "avg of three scorers",
			scorers: []scorer.Scorer{fileScorer, depScorer, linesScorer},
			reduce:  Avg,
			stats:   scorer.ChangeStats{FilesChanged: 3, DependencyCount: 5, LinesAdded: 50},
			want:    0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.scorers, tt.reduce)
			require.NoError(t, err)
			got, err := s.Score(context.Background(), tt.stats)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 1e-9)
		})
	}
}

// errorScorer always returns an error.
type errorScorer struct{}

func (e *errorScorer) Score(_ context.Context, _ scorer.ChangeStats) (float64, error) {
	return 0, fmt.Errorf("scorer failed")
}

func TestScorer_Score_ErrorFromFirstScorer(t *testing.T) {
	h, err := heuristic.New([]heuristic.Bucket{
		{Min: 0, Max: 1000, Score: 0.9},
	}, heuristic.FilesChanged)
	require.NoError(t, err)

	s, err := New([]scorer.Scorer{&errorScorer{}, h}, Min)
	require.NoError(t, err)
	_, err = s.Score(context.Background(), scorer.ChangeStats{})
	require.Error(t, err)
}

func TestNew_EmptyScorers(t *testing.T) {
	_, err := New([]scorer.Scorer{}, Min)
	require.Error(t, err)
}

func TestNew_NilReduce(t *testing.T) {
	h, err := heuristic.New([]heuristic.Bucket{
		{Min: 0, Max: 1000, Score: 0.9},
	}, heuristic.FilesChanged)
	require.NoError(t, err)

	_, err = New([]scorer.Scorer{h}, nil)
	require.Error(t, err)
}
