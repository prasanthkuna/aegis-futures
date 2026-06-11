package backtest

import (
	"context"
	"fmt"
	"time"
)

// Dataset is preloaded kline history shared across many backtest runs.
type Dataset struct {
	All      map[string][]Bar
	Context  map[string]ContextSeries
	Timeline []time.Time
	IdxMaps  map[string]map[int64]int
	OOSStart time.Time
	Days     int
}

func (r *Runner) LoadDataset(ctx context.Context, days int, cachedOnly bool) (*Dataset, error) {
	if days <= 0 {
		days = 60
	}
	loadCtx := ctx
	var symbols []string
	if cachedOnly {
		loadCtx = context.WithValue(ctx, cachedOnlyKey{}, true)
		symbols = r.Store.ListCached(days)
		if len(symbols) == 0 {
			return nil, fmt.Errorf("no cached klines in %s", r.Store.CacheDir)
		}
	} else {
		var err error
		symbols, err = r.Store.TopSymbolsByVolume(ctx, 50)
		if err != nil {
			return nil, err
		}
	}
	all := make(map[string][]Bar, len(symbols))
	ctxAll := make(map[string]ContextSeries, len(symbols))
	for _, sym := range symbols {
		bars, err := r.Store.LoadOrFetch(loadCtx, sym, days)
		if err != nil {
			continue
		}
		if len(bars) > warmupBars {
			all[sym] = bars
		}
		if cs, err := r.Store.LoadContext(sym, days); err == nil && len(cs) > 0 {
			ctxAll[sym] = cs
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no bar data loaded")
	}
	timeline := BuildTimeline(all)
	if len(timeline) <= warmupBars+10 {
		return nil, fmt.Errorf("timeline too short")
	}
	oosFrac := 0.7
	return &Dataset{
		All: all, Context: ctxAll, Timeline: timeline, IdxMaps: IndexMap(all),
		OOSStart: timeline[int(float64(len(timeline))*oosFrac)],
		Days:     days,
	}, nil
}

// FilterDataset keeps only the given symbols (rebuilds timeline/index).
func FilterDataset(ds *Dataset, symbols []string) (*Dataset, error) {
	if ds == nil {
		return nil, fmt.Errorf("nil dataset")
	}
	want := map[string]bool{}
	for _, s := range symbols {
		want[s] = true
	}
	all := make(map[string][]Bar, len(symbols))
	ctx := make(map[string]ContextSeries, len(symbols))
	for sym, bars := range ds.All {
		if !want[sym] {
			continue
		}
		all[sym] = bars
		if ds.Context != nil {
			if c, ok := ds.Context[sym]; ok {
				ctx[sym] = c
			}
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no data for symbols %v", symbols)
	}
	timeline := BuildTimeline(all)
	if len(timeline) <= warmupBars+10 {
		return nil, fmt.Errorf("timeline too short for %v", symbols)
	}
	oosFrac := 0.7
	return &Dataset{
		All: all, Context: ctx, Timeline: timeline, IdxMaps: IndexMap(all),
		OOSStart: timeline[int(float64(len(timeline))*oosFrac)],
		Days:     ds.Days,
	}, nil
}
