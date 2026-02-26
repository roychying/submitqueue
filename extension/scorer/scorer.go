package scorer

//go:generate mockgen -source=scorer.go -destination=mock/scorer.go -package=mock

import "context"

// ChangeStats contains aggregate statistics about a code change used for scoring.
type ChangeStats struct {
	// FilesChanged is the number of files modified.
	FilesChanged int
	// LinesAdded is the total lines added across all files.
	LinesAdded int
	// LinesDeleted is the total lines deleted across all files.
	LinesDeleted int
	// LinesModified is the total lines modified across all files.
	LinesModified int

	// BuildTargetsAdded is the number of build targets added.
	BuildTargetsAdded int
	// BuildTargetsRemoved is the number of build targets removed.
	BuildTargetsRemoved int
	// BuildTargetsChanged is the number of build targets modified.
	BuildTargetsChanged int
	// DependencyCount is the number of downstream dependencies affected.
	DependencyCount int
}

// Scorer computes a success probability score for a change based on its characteristics.
type Scorer interface {
	// Score returns a probability between 0.0 and 1.0 indicating the likelihood
	// of a successful land for a change with the given statistics.
	Score(ctx context.Context, stats ChangeStats) (float64, error)
}
