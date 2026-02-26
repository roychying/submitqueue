package heuristic

import (
	"context"
	"fmt"

	"github.com/uber/submitqueue/extension/scorer"
)

// Bucket defines a range [Min, Max] mapped to a probability Score.
type Bucket struct {
	// Min is the inclusive lower bound of the range.
	Min int
	// Max is the inclusive upper bound of the range.
	Max int
	// Score is the probability returned when the metric falls within this bucket.
	Score float64
}

// StatFunc extracts a single numeric value from ChangeStats for bucketing.
type StatFunc func(scorer.ChangeStats) int

// FilesChanged returns the number of files changed from ChangeStats.
func FilesChanged(stats scorer.ChangeStats) int { return stats.FilesChanged }

// LinesAdded returns the number of lines added from ChangeStats.
func LinesAdded(stats scorer.ChangeStats) int { return stats.LinesAdded }

// LinesDeleted returns the number of lines deleted from ChangeStats.
func LinesDeleted(stats scorer.ChangeStats) int { return stats.LinesDeleted }

// LinesModified returns the number of lines modified from ChangeStats.
func LinesModified(stats scorer.ChangeStats) int { return stats.LinesModified }

// BuildTargetsAdded returns the number of build targets added from ChangeStats.
func BuildTargetsAdded(stats scorer.ChangeStats) int { return stats.BuildTargetsAdded }

// BuildTargetsRemoved returns the number of build targets removed from ChangeStats.
func BuildTargetsRemoved(stats scorer.ChangeStats) int { return stats.BuildTargetsRemoved }

// BuildTargetsChanged returns the number of build targets modified from ChangeStats.
func BuildTargetsChanged(stats scorer.ChangeStats) int { return stats.BuildTargetsChanged }

// DependencyCount returns the number of downstream dependencies affected from ChangeStats.
func DependencyCount(stats scorer.ChangeStats) int { return stats.DependencyCount }

// heuristicScorer computes a success probability by bucketing a metric extracted from ChangeStats.
// It follows the Java HeuristicsBasedSuccessPredictor pattern.
type heuristicScorer struct {
	// buckets is the list of ranges to match against.
	buckets []Bucket
	// statFunc extracts the numeric metric from ChangeStats.
	statFunc StatFunc
}

// New creates a new heuristic Scorer with the given buckets and stat function.
// Returns an error if statFunc is nil.
func New(buckets []Bucket, statFunc StatFunc) (scorer.Scorer, error) {
	if statFunc == nil {
		return nil, fmt.Errorf("heuristic.New: statFunc must not be nil")
	}
	return &heuristicScorer{
		buckets:  buckets,
		statFunc: statFunc,
	}, nil
}

// Score returns the probability score for the first bucket whose range [Min, Max]
// contains the extracted metric value. Returns an error if no bucket matches.
func (s *heuristicScorer) Score(_ context.Context, stats scorer.ChangeStats) (float64, error) {
	metric := s.statFunc(stats)
	for _, b := range s.buckets {
		if metric >= b.Min && metric <= b.Max {
			return b.Score, nil
		}
	}
	return 0, fmt.Errorf("no bucket matches metric value %d", metric)
}
