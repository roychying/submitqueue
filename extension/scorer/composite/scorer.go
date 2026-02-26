package composite

import (
	"context"
	"fmt"
	"slices"

	"github.com/uber/submitqueue/extension/scorer"
)

// ReduceFunc combines multiple scores into a single score.
type ReduceFunc func([]float64) float64

// Min returns the minimum value from scores.
func Min(scores []float64) float64 { return slices.Min(scores) }

// Max returns the maximum value from scores.
func Max(scores []float64) float64 { return slices.Max(scores) }

// Avg returns the arithmetic mean of scores.
func Avg(scores []float64) float64 {
	var sum float64
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}

// compositeScorer combines multiple scorers into a single score using a reduce function.
type compositeScorer struct {
	// scorers is the list of child scorers to evaluate.
	scorers []scorer.Scorer
	// reduce combines individual scores into a single value.
	reduce ReduceFunc
}

// New creates a composite Scorer that evaluates all child scorers and combines
// their results using the given reduce function.
// Returns an error if scorers is empty or reduce is nil.
func New(scorers []scorer.Scorer, reduce ReduceFunc) (scorer.Scorer, error) {
	if len(scorers) == 0 {
		return nil, fmt.Errorf("composite.New: scorers must not be empty")
	}
	if reduce == nil {
		return nil, fmt.Errorf("composite.New: reduce must not be nil")
	}
	return &compositeScorer{
		scorers: scorers,
		reduce:  reduce,
	}, nil
}

// Score evaluates all child scorers and combines their results using the reduce function.
// If any child scorer returns an error, that error is returned immediately.
func (c *compositeScorer) Score(ctx context.Context, stats scorer.ChangeStats) (float64, error) {
	scores := make([]float64, len(c.scorers))
	for i, s := range c.scorers {
		score, err := s.Score(ctx, stats)
		if err != nil {
			return 0, err
		}
		scores[i] = score
	}
	return c.reduce(scores), nil
}
