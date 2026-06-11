package engine

import (
	"fmt"
	"time"

	"encore.app/model"
	"encore.app/signal"
)

const maxFeed = 60

func (rt *Runtime) recordScan(out signal.RankOutput) {
	rt.feedMu.Lock()
	defer rt.feedMu.Unlock()

	rt.feed = append(rt.feed, model.SignalFeedEvent{
		At: nowUTC(), Kind: "scan",
		Message: fmt.Sprintf(
			"scanned %d · %d above floor · %d near-miss · %d will-fire · max %d",
			out.Heartbeat.SymbolsScanned, out.Heartbeat.AboveFloor,
			out.Heartbeat.NearMissCount, out.Heartbeat.WillFireCount,
			out.Heartbeat.MaxStrength,
		),
	})

	for _, s := range out.Universe {
		if !s.WillFire {
			continue
		}
		rt.feed = append(rt.feed, model.SignalFeedEvent{
			At: nowUTC(), Kind: "will_fire", Symbol: s.Symbol,
			Strength: s.Strength, Playbook: s.Playbook,
			Message: fmt.Sprintf("%s %s str %d — auto entry eligible", s.Symbol, s.Side, s.Strength),
		})
	}
	if len(rt.feed) > maxFeed {
		rt.feed = rt.feed[len(rt.feed)-maxFeed:]
	}
}

func (rt *Runtime) recordExecute(sym string, ok bool, msg string) {
	rt.feedMu.Lock()
	defer rt.feedMu.Unlock()
	kind := "execute_ok"
	if !ok {
		kind = "execute_fail"
	}
	rt.feed = append(rt.feed, model.SignalFeedEvent{
		At: nowUTC(), Kind: kind, Symbol: sym, Message: msg,
	})
	if len(rt.feed) > maxFeed {
		rt.feed = rt.feed[len(rt.feed)-maxFeed:]
	}
}

func (rt *Runtime) FeedEvents() []model.SignalFeedEvent {
	rt.feedMu.RLock()
	defer rt.feedMu.RUnlock()
	out := make([]model.SignalFeedEvent, len(rt.feed))
	copy(out, rt.feed)
	return out
}

func (rt *Runtime) LastRank() signal.RankOutput {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.lastRank
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
