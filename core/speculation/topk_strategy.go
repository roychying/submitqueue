package speculation

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/uber/submitqueue/entity"
)

// DefaultK is the default number of top paths to generate when k <= 0.
const DefaultK = 32

// MaxTopKDependencies is the maximum number of predecessor dependencies allowed
// for top-K path generation. Unlike the brute-force generateTree (capped at 10),
// the top-K algorithm is efficient up to much larger dependency counts because it
// generates only K paths instead of all 2^N.
const MaxTopKDependencies = 50

// ProbabilityFunc provides success probabilities for dependency batch IDs.
// Implementations may use historical data, ML models, or static heuristics.
// The returned map must have an entry for every input ID.
type ProbabilityFunc func(ctx context.Context, dependencyIDs []string) (map[string]float64, error)

// TopKStrategy generates the top-K highest-probability speculation paths.
// It uses an optional ProbabilityFunc to obtain success probabilities for
// each dependency. If no function is provided, defaults to 0.5 for all.
type TopKStrategy struct {
	// probFn provides per-dependency success probabilities. May be nil.
	probFn ProbabilityFunc
	// k is the number of top paths to generate.
	k int
	// minScore is the minimum path score threshold (in product space, 0 to 1).
	// Paths with scores below this threshold are not generated.
	// A value of 0 disables the threshold (all K paths are returned).
	minScore float32
}

// Verify TopKStrategy implements Strategy at compile time.
var _ Strategy = (*TopKStrategy)(nil)

// NewTopKStrategy creates a new TopKStrategy that generates at most k
// speculation paths with scores >= minScore. If probFn is nil, all
// dependencies default to 0.5. If k <= 0, DefaultK is used.
// If minScore <= 0, no threshold is applied.
func NewTopKStrategy(probFn ProbabilityFunc, k int, minScore float32) *TopKStrategy {
	return &TopKStrategy{
		probFn:   probFn,
		k:        k,
		minScore: minScore,
	}
}

// Generate produces a speculation tree by obtaining dependency probabilities
// and selecting the top-K paths. If the probability function fails, it falls
// back to default probabilities of 0.5 for all dependencies.
func (s *TopKStrategy) Generate(ctx context.Context, batchID string, dependencyIDs []string) (entity.SpeculationTree, error) {
	probabilities := defaultProbabilities(dependencyIDs)

	if s.probFn != nil && len(dependencyIDs) > 0 {
		scored, err := s.probFn(ctx, dependencyIDs)
		if err != nil {
			// Fall back to default probabilities on failure.
			// This is a soft error — speculation continues with uniform priors.
		} else {
			probabilities = scored
		}
	}

	tree, err := generateTopK(batchID, dependencyIDs, probabilities, s.k, s.minScore)
	if err != nil {
		return entity.SpeculationTree{}, fmt.Errorf("top-k generation failed: %w", err)
	}

	return tree, nil
}

// defaultProbabilities returns a map with 0.5 probability for all IDs.
func defaultProbabilities(ids []string) map[string]float64 {
	probs := make(map[string]float64, len(ids))
	for _, id := range ids {
		probs[id] = 0.5
	}
	return probs
}

// flipEntry represents a candidate path in the top-K enumeration.
// Each entry corresponds to a set of dependency flips relative to the optimal path.
type flipEntry struct {
	// cost is the total flip cost (sum of individual flip costs for all flipped deps).
	cost float64
	// lastIndex is the sorted index of the last flipped dependency.
	lastIndex int
	// flipped contains the sorted indices (in flip-cost order) of dependencies
	// whose inclusion is toggled from the optimal path.
	flipped []int
}

// flipHeap is a min-heap of flipEntry ordered by ascending cost.
// Lower cost means higher probability path.
type flipHeap []flipEntry

var _ heap.Interface = (*flipHeap)(nil)

func (h flipHeap) Len() int           { return len(h) }
func (h flipHeap) Less(i, j int) bool { return h[i].cost < h[j].cost }
func (h flipHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *flipHeap) Push(x any) {
	*h = append(*h, x.(flipEntry))
}

func (h *flipHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	*h = old[:n-1]
	return entry
}

// sortedDep holds per-dependency metadata ordered by ascending flip cost.
type sortedDep struct {
	originalIndex     int
	flipCost          float64
	optimallyIncluded bool
}

// generateTopK produces the top-K highest-probability speculation paths for a batch.
//
// Given success probabilities for each predecessor, it finds the optimal path
// (include dep i if P_i >= 0.5) and enumerates deviations in ascending cost order
// using a min-heap. This yields paths in descending probability order without
// generating all 2^N paths.
//
// Paths with scores below minScore are not generated. When minScore <= 0, no
// threshold is applied and all K paths are returned.
//
// Complexity: O(N log N + K log K) time, O(K) space.
//
// If k <= 0, DefaultK is used. If k >= 2^N, all paths are returned.
// Returns an error if len(dependencyIDs) exceeds MaxTopKDependencies.
func generateTopK(
	currentID string,
	dependencyIDs []string,
	probabilities map[string]float64,
	k int,
	minScore float32,
) (entity.SpeculationTree, error) {
	n := len(dependencyIDs)
	if n > MaxTopKDependencies {
		return entity.SpeculationTree{}, fmt.Errorf(
			"dependency count %d exceeds maximum %d", n, MaxTopKDependencies,
		)
	}

	if k <= 0 {
		k = DefaultK
	}

	// Defensive copy to avoid mutation of caller's slice.
	deps := make([]string, n)
	copy(deps, dependencyIDs)

	// No dependencies: single path with empty base.
	if n == 0 {
		return entity.SpeculationTree{
			BatchID: currentID,
			Speculations: []entity.SpeculationInfo{
				{
					Path: entity.SpeculationPath{
						Base: []string{},
						Head: currentID,
					},
					Action: entity.SpeculationPathActionSchedule,
					Score:  1.0,
				},
			},
		}, nil
	}

	const epsilon = 1e-9

	sorted := make([]sortedDep, n)
	optimalLogScore := 0.0

	for i, depID := range deps {
		p := 0.5
		if prob, ok := probabilities[depID]; ok {
			p = prob
		}
		// Clamp to [epsilon, 1-epsilon] to avoid log(0).
		p = math.Max(epsilon, math.Min(1-epsilon, p))

		included := p >= 0.5
		preferred := math.Max(p, 1-p)
		nonPreferred := math.Min(p, 1-p)

		optimalLogScore += math.Log(preferred)

		sorted[i] = sortedDep{
			originalIndex:     i,
			flipCost:          math.Log(preferred / nonPreferred),
			optimallyIncluded: included,
		}
	}

	// Sort by ascending flip cost (cheapest flips first).
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].flipCost < sorted[j].flipCost
	})

	// Cap K to 2^N when computable.
	if n < 63 {
		if maxPaths := 1 << n; k > maxPaths {
			k = maxPaths
		}
	}

	// Compute maximum flip cost from minScore threshold.
	// score = exp(optimalLogScore - flipCost) >= minScore
	// flipCost <= optimalLogScore - log(minScore)
	maxFlipCost := math.Inf(1) // no limit by default
	if minScore > 0 {
		maxFlipCost = optimalLogScore - math.Log(float64(minScore))
	}

	// buildSpec constructs a SpeculationInfo from a set of flipped sorted indices.
	buildSpec := func(flipped []int, totalFlipCost float64) entity.SpeculationInfo {
		included := make([]bool, n)
		for _, sd := range sorted {
			included[sd.originalIndex] = sd.optimallyIncluded
		}
		for _, sortedIdx := range flipped {
			origIdx := sorted[sortedIdx].originalIndex
			included[origIdx] = !included[origIdx]
		}

		base := make([]string, 0)
		for i, dep := range deps {
			if included[i] {
				base = append(base, dep)
			}
		}

		score := float32(math.Exp(optimalLogScore - totalFlipCost))
		return entity.SpeculationInfo{
			Path: entity.SpeculationPath{
				Base: base,
				Head: currentID,
			},
			Action: entity.SpeculationPathActionSchedule,
			Score:  score,
		}
	}

	speculations := make([]entity.SpeculationInfo, 0, k)

	// Result #1: the optimal path (no flips).
	speculations = append(speculations, buildSpec(nil, 0))

	if k == 1 {
		return entity.SpeculationTree{
			BatchID:      currentID,
			Speculations: speculations,
		}, nil
	}

	// Min-heap enumeration of subsets in ascending flip cost order.
	h := &flipHeap{}
	heap.Init(h)

	// Only seed the heap if the first flip is within the cost threshold.
	if sorted[0].flipCost <= maxFlipCost {
		heap.Push(h, flipEntry{
			cost:      sorted[0].flipCost,
			lastIndex: 0,
			flipped:   []int{0},
		})
	}

	for h.Len() > 0 && len(speculations) < k {
		entry := heap.Pop(h).(flipEntry)

		// All subsequent entries have equal or higher cost, so stop.
		if entry.cost > maxFlipCost {
			break
		}

		speculations = append(speculations, buildSpec(entry.flipped, entry.cost))

		j := entry.lastIndex
		if j+1 < n {
			// Extend: also flip dep j+1.
			extCost := entry.cost + sorted[j+1].flipCost
			if extCost <= maxFlipCost {
				extFlipped := make([]int, len(entry.flipped)+1)
				copy(extFlipped, entry.flipped)
				extFlipped[len(entry.flipped)] = j + 1

				heap.Push(h, flipEntry{
					cost:      extCost,
					lastIndex: j + 1,
					flipped:   extFlipped,
				})
			}

			// Swap: unflip dep j, flip dep j+1 instead.
			swapCost := entry.cost - sorted[j].flipCost + sorted[j+1].flipCost
			if swapCost <= maxFlipCost {
				swapFlipped := make([]int, len(entry.flipped))
				copy(swapFlipped, entry.flipped)
				swapFlipped[len(swapFlipped)-1] = j + 1

				heap.Push(h, flipEntry{
					cost:      swapCost,
					lastIndex: j + 1,
					flipped:   swapFlipped,
				})
			}
		}
	}

	return entity.SpeculationTree{
		BatchID:      currentID,
		Speculations: speculations,
	}, nil
}
