package backtest

import "time"

// WFAFold is one purged out-of-sample test window on the shared timeline.
type WFAFold struct {
	Index     int
	TestStart time.Time
	TestEnd   time.Time
}

// BuildWFAFolds splits the post-warmup timeline into nFolds test windows with purge gaps.
// Tests occupy the range [testStartFrac, holdoutFrac) of the timeline.
func BuildWFAFolds(timeline []time.Time, nFolds int, testStartFrac, holdoutFrac, purgeFrac float64) []WFAFold {
	if len(timeline) < 100 || nFolds < 1 {
		return nil
	}
	if testStartFrac <= 0 {
		testStartFrac = 0.55
	}
	if holdoutFrac <= testStartFrac {
		holdoutFrac = 0.85
	}
	if purgeFrac <= 0 {
		purgeFrac = 0.02
	}
	n := len(timeline)
	testStartIdx := int(float64(n) * testStartFrac)
	holdoutIdx := int(float64(n) * holdoutFrac)
	testSpan := holdoutIdx - testStartIdx
	if testSpan <= nFolds {
		return nil
	}
	purgeBars := int(float64(n) * purgeFrac)
	if purgeBars < 1 {
		purgeBars = 1
	}
	foldLen := testSpan / nFolds

	var folds []WFAFold
	cursor := testStartIdx
	for i := 0; i < nFolds; i++ {
		start := cursor
		end := start + foldLen
		if i == nFolds-1 {
			end = holdoutIdx
		}
		if end > len(timeline) {
			end = len(timeline)
		}
		if start >= end {
			break
		}
		folds = append(folds, WFAFold{
			Index:     i + 1,
			TestStart: timeline[start],
			TestEnd:   timeline[end-1].Add(5 * time.Minute),
		})
		cursor = end + purgeBars
		if cursor >= holdoutIdx {
			break
		}
	}
	return folds
}

// HoldoutWindow returns the final untouched segment [holdoutFrac, 1.0).
func HoldoutWindow(timeline []time.Time, holdoutFrac float64) (start, end time.Time) {
	if len(timeline) == 0 {
		return
	}
	if holdoutFrac <= 0 {
		holdoutFrac = 0.85
	}
	idx := int(float64(len(timeline)) * holdoutFrac)
	if idx >= len(timeline) {
		idx = len(timeline) - 1
	}
	return timeline[idx], timeline[len(timeline)-1].Add(5 * time.Minute)
}

const proWFAMinFoldPnL = 3.0

// EvaluateWFA scores how many folds pass on a single full-period backtest result.
func EvaluateWFA(res *Result, folds []WFAFold, minPasses int) (passes int, metrics []FoldMetrics) {
	if res == nil {
		return 0, nil
	}
	for _, f := range folds {
		m := res.OOSMetricsBetween(f.TestStart, f.TestEnd)
		metrics = append(metrics, m)
		if m.Passed && m.OOSNetPnL >= proWFAMinFoldPnL {
			passes++
		}
	}
	if minPasses <= 0 {
		minPasses = (len(folds)*3 + 3) / 4 // majority: ceil(3/4 * n)
	}
	return passes, metrics
}

func wfaPassed(passes, total, minPasses int) bool {
	if total == 0 {
		return false
	}
	if minPasses <= 0 {
		minPasses = (total*3 + 3) / 4
	}
	return passes >= minPasses
}
