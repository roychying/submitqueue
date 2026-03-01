package speculation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/submitqueue/entity"
)

// pathKey creates a string representation of a path for set comparison.
func pathKey(p entity.SpeculationPath) string {
	return fmt.Sprintf("%v->%s", p.Base, p.Head)
}

// pathSet extracts the set of path keys from speculation infos.
func pathSet(specs []entity.SpeculationInfo) map[string]bool {
	set := make(map[string]bool, len(specs))
	for _, s := range specs {
		set[pathKey(s.Path)] = true
	}
	return set
}

func TestgenerateTopK(t *testing.T) {
	tests := []struct {
		name      string
		currentID string
		deps      []string
		probs     map[string]float64
		k         int
		wantCount int
		wantPaths []entity.SpeculationPath // nil to skip ordered path comparison
	}{
		{
			name:      "zero dependencies",
			currentID: "B1",
			deps:      nil,
			probs:     nil,
			k:         10,
			wantCount: 1,
			wantPaths: []entity.SpeculationPath{
				{Base: []string{}, Head: "B1"},
			},
		},
		{
			name:      "one dependency high probability",
			currentID: "B2",
			deps:      []string{"B1"},
			probs:     map[string]float64{"B1": 0.9},
			k:         10,
			wantCount: 2,
			wantPaths: []entity.SpeculationPath{
				{Base: []string{"B1"}, Head: "B2"}, // optimal: include B1 (P=0.9)
				{Base: []string{}, Head: "B2"},      // flip: exclude B1
			},
		},
		{
			name:      "one dependency low probability",
			currentID: "B2",
			deps:      []string{"B1"},
			probs:     map[string]float64{"B1": 0.1},
			k:         10,
			wantCount: 2,
			wantPaths: []entity.SpeculationPath{
				{Base: []string{}, Head: "B2"},      // optimal: exclude B1 (P=0.1)
				{Base: []string{"B1"}, Head: "B2"}, // flip: include B1
			},
		},
		{
			name:      "two dependencies",
			currentID: "B3",
			deps:      []string{"B1", "B2"},
			probs:     map[string]float64{"B1": 0.9, "B2": 0.8},
			k:         10,
			wantCount: 4,
		},
		{
			name:      "three dependencies",
			currentID: "B4",
			deps:      []string{"B1", "B2", "B3"},
			probs:     map[string]float64{"B1": 0.9, "B2": 0.7, "B3": 0.6},
			k:         10,
			wantCount: 8,
		},
		{
			name:      "equal probabilities 0.5",
			currentID: "B3",
			deps:      []string{"B1", "B2"},
			probs:     map[string]float64{"B1": 0.5, "B2": 0.5},
			k:         10,
			wantCount: 4,
		},
		{
			name:      "missing probabilities default to 0.5",
			currentID: "B3",
			deps:      []string{"B1", "B2"},
			probs:     map[string]float64{},
			k:         10,
			wantCount: 4,
		},
		{
			name:      "probability near zero",
			currentID: "B2",
			deps:      []string{"B1"},
			probs:     map[string]float64{"B1": 0.001},
			k:         10,
			wantCount: 2,
			wantPaths: []entity.SpeculationPath{
				{Base: []string{}, Head: "B2"},      // optimal: exclude
				{Base: []string{"B1"}, Head: "B2"}, // flip: include
			},
		},
		{
			name:      "probability near one",
			currentID: "B2",
			deps:      []string{"B1"},
			probs:     map[string]float64{"B1": 0.999},
			k:         10,
			wantCount: 2,
			wantPaths: []entity.SpeculationPath{
				{Base: []string{"B1"}, Head: "B2"}, // optimal: include
				{Base: []string{}, Head: "B2"},      // flip: exclude
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := generateTopK(tt.currentID, tt.deps, tt.probs, tt.k, 0)
			require.NoError(t, err)

			assert.Equal(t, tt.currentID, tree.BatchID)
			require.Len(t, tree.Speculations, tt.wantCount)

			if tt.wantPaths != nil {
				for i, spec := range tree.Speculations {
					assert.Equal(t, tt.wantPaths[i], spec.Path, "path at index %d", i)
				}
			}

			// Verify all paths have Head = currentID and Action = Schedule.
			for i, spec := range tree.Speculations {
				assert.Equal(t, tt.currentID, spec.Path.Head, "head at index %d", i)
				assert.Equal(t, entity.SpeculationPathActionSchedule, spec.Action, "action at index %d", i)
			}
		})
	}
}

func TestgenerateTopK_DescendingScoreOrder(t *testing.T) {
	probs := map[string]float64{"B1": 0.9, "B2": 0.7, "B3": 0.6}
	tree, err := generateTopK("B4", []string{"B1", "B2", "B3"}, probs, 8, 0)
	require.NoError(t, err)

	for i := 1; i < len(tree.Speculations); i++ {
		assert.GreaterOrEqual(t, tree.Speculations[i-1].Score, tree.Speculations[i].Score,
			"score at index %d should be >= score at index %d", i-1, i)
	}
}

func TestgenerateTopK_KLessThanTotal(t *testing.T) {
	deps := []string{"B1", "B2", "B3", "B4"}
	probs := map[string]float64{"B1": 0.9, "B2": 0.8, "B3": 0.7, "B4": 0.6}

	tree, err := generateTopK("B5", deps, probs, 5, 0)
	require.NoError(t, err)

	assert.Len(t, tree.Speculations, 5)
}

func TestgenerateTopK_KGreaterThanTotal(t *testing.T) {
	deps := []string{"B1", "B2"}
	probs := map[string]float64{"B1": 0.9, "B2": 0.8}

	tree, err := generateTopK("B3", deps, probs, 100, 0)
	require.NoError(t, err)

	assert.Len(t, tree.Speculations, 4) // 2^2 = 4
}

func TestgenerateTopK_DefaultK(t *testing.T) {
	// With 6 deps, 2^6 = 64 > DefaultK = 32.
	deps := make([]string, 6)
	probs := make(map[string]float64, 6)
	for i := range deps {
		deps[i] = fmt.Sprintf("B%d", i+1)
		probs[deps[i]] = 0.8
	}

	tree, err := generateTopK("current", deps, probs, 0, 0)
	require.NoError(t, err)

	assert.Len(t, tree.Speculations, DefaultK)
}

func TestgenerateTopK_ExceedsMaxDependencies(t *testing.T) {
	deps := make([]string, MaxTopKDependencies+1)
	for i := range deps {
		deps[i] = fmt.Sprintf("B%d", i+1)
	}

	_, err := generateTopK("current", deps, nil, DefaultK, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum")
}

func TestgenerateTopK_EquivalenceWithgenerateTree(t *testing.T) {
	tests := []struct {
		name  string
		deps  []string
		probs map[string]float64
	}{
		{
			name:  "zero deps",
			deps:  nil,
			probs: nil,
		},
		{
			name:  "one dep",
			deps:  []string{"B1"},
			probs: map[string]float64{"B1": 0.7},
		},
		{
			name:  "two deps",
			deps:  []string{"B1", "B2"},
			probs: map[string]float64{"B1": 0.8, "B2": 0.6},
		},
		{
			name:  "three deps",
			deps:  []string{"B1", "B2", "B3"},
			probs: map[string]float64{"B1": 0.9, "B2": 0.7, "B3": 0.6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := len(tt.deps)
			allPaths := 1 << n

			bruteForce, err := generateTree("current", tt.deps)
			require.NoError(t, err)

			topK, err := generateTopK("current", tt.deps, tt.probs, allPaths, 0)
			require.NoError(t, err)

			// Same number of paths.
			require.Len(t, topK.Speculations, len(bruteForce.Speculations))

			// Same set of paths (regardless of order).
			assert.Equal(t, pathSet(bruteForce.Speculations), pathSet(topK.Speculations))
		})
	}
}

func TestgenerateTopK_ScoreValues(t *testing.T) {
	// Single dep with known probability: scores should match expected values.
	probs := map[string]float64{"B1": 0.8}
	tree, err := generateTopK("B2", []string{"B1"}, probs, 2, 0)
	require.NoError(t, err)

	require.Len(t, tree.Speculations, 2)
	// Optimal: include B1, score = 0.8
	assert.InDelta(t, 0.8, float64(tree.Speculations[0].Score), 1e-6)
	// Flipped: exclude B1, score = 0.2
	assert.InDelta(t, 0.2, float64(tree.Speculations[1].Score), 1e-6)
}

func TestgenerateTopK_InputImmutability(t *testing.T) {
	deps := []string{"B1", "B2", "B3"}
	original := make([]string, len(deps))
	copy(original, deps)

	probs := map[string]float64{"B1": 0.9, "B2": 0.8, "B3": 0.7}
	_, err := generateTopK("B4", deps, probs, 8, 0)
	require.NoError(t, err)

	assert.Equal(t, original, deps, "input dependency slice should not be mutated")
}

func TestgenerateTopK_LargeDependencyCount(t *testing.T) {
	n := 30
	deps := make([]string, n)
	probs := make(map[string]float64, n)
	for i := range deps {
		deps[i] = fmt.Sprintf("B%d", i+1)
		probs[deps[i]] = 0.8
	}

	tree, err := generateTopK("current", deps, probs, 32, 0)
	require.NoError(t, err)

	assert.Len(t, tree.Speculations, 32)

	// Verify descending score order.
	for i := 1; i < len(tree.Speculations); i++ {
		assert.GreaterOrEqual(t, tree.Speculations[i-1].Score, tree.Speculations[i].Score,
			"score at index %d should be >= score at index %d", i-1, i)
	}
}

func TestGenerateTopK_MinScore(t *testing.T) {
	tests := []struct {
		name         string
		deps         []string
		probs        map[string]float64
		k            int
		minScore     float32
		wantMaxCount int // upper bound on path count
		wantMinCount int // lower bound on path count
	}{
		{
			name:         "no threshold returns all paths",
			deps:         []string{"B1", "B2"},
			probs:        map[string]float64{"B1": 0.9, "B2": 0.8},
			k:            10,
			minScore:     0,
			wantMaxCount: 4, // 2^2
			wantMinCount: 4,
		},
		{
			name:         "high threshold returns fewer paths",
			deps:         []string{"B1", "B2"},
			probs:        map[string]float64{"B1": 0.9, "B2": 0.8},
			k:            10,
			minScore:     0.5,
			wantMaxCount: 1, // only optimal path (0.9 * 0.8 = 0.72) passes
			wantMinCount: 1,
		},
		{
			name:         "moderate threshold filters low-probability paths",
			deps:         []string{"B1", "B2"},
			probs:        map[string]float64{"B1": 0.9, "B2": 0.8},
			k:            10,
			minScore:     0.1,
			wantMaxCount: 3, // 0.72, 0.18, 0.08 pass; 0.02 filtered
			wantMinCount: 2,
		},
		{
			name:         "threshold above optimal score returns only optimal",
			deps:         []string{"B1"},
			probs:        map[string]float64{"B1": 0.6},
			k:            10,
			minScore:     0.5,
			wantMaxCount: 1, // optimal = 0.6 passes, flip = 0.4 filtered
			wantMinCount: 1,
		},
		{
			name:         "zero deps ignores threshold",
			deps:         nil,
			probs:        nil,
			k:            10,
			minScore:     0.99,
			wantMaxCount: 1,
			wantMinCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := generateTopK("current", tt.deps, tt.probs, tt.k, tt.minScore)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, len(tree.Speculations), tt.wantMinCount,
				"should have at least %d paths", tt.wantMinCount)
			assert.LessOrEqual(t, len(tree.Speculations), tt.wantMaxCount,
				"should have at most %d paths", tt.wantMaxCount)

			// All returned paths must have scores >= minScore.
			for i, spec := range tree.Speculations {
				if tt.minScore > 0 {
					assert.GreaterOrEqual(t, spec.Score, tt.minScore,
						"path %d score %f should be >= minScore %f", i, spec.Score, tt.minScore)
				}
			}
		})
	}
}

func TestGenerateTopK_MinScoreAllPathsAboveThreshold(t *testing.T) {
	// With 1 dep at 0.5, optimal score = 0.5, flip score = 0.5.
	// Both paths should be returned with minScore = 0.5.
	probs := map[string]float64{"B1": 0.5}
	tree, err := generateTopK("B2", []string{"B1"}, probs, 10, 0.5)
	require.NoError(t, err)

	assert.Len(t, tree.Speculations, 2)
	for _, spec := range tree.Speculations {
		assert.InDelta(t, 0.5, float64(spec.Score), 1e-6)
	}
}

func TestGenerateTopK_MinScoreVsKInteraction(t *testing.T) {
	// K=10 but minScore filters more aggressively than K.
	// 3 deps at 0.9: optimal = 0.729, and paths drop quickly.
	deps := []string{"B1", "B2", "B3"}
	probs := map[string]float64{"B1": 0.9, "B2": 0.9, "B3": 0.9}

	tree, err := generateTopK("B4", deps, probs, 10, 0.05)
	require.NoError(t, err)

	// All returned paths should be above the threshold.
	for i, spec := range tree.Speculations {
		assert.GreaterOrEqual(t, spec.Score, float32(0.05),
			"path %d score %f should be >= 0.05", i, spec.Score)
	}

	// Should have fewer paths than without threshold.
	treeNoThreshold, err := generateTopK("B4", deps, probs, 10, 0)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(tree.Speculations), len(treeNoThreshold.Speculations))
}
