package aegis

import (
	"sync"

	"encore.app/model"
)

type radarPrevSnap struct {
	Score       float64
	Price       float64
	VolumeSurge float64
}

type radarCache struct {
	mu   sync.Mutex
	prev map[string]radarPrevSnap
}

func (c *radarCache) applyDeltas(items []model.SymbolSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prev == nil {
		c.prev = make(map[string]radarPrevSnap)
	}
	for i := range items {
		it := &items[i]
		if p, ok := c.prev[it.Symbol]; ok {
			it.ScoreDelta = it.TradeScore - p.Score
			if p.Price > 0 {
				it.PriceDeltaPct = (it.Price - p.Price) / p.Price * 100
			}
			it.SurgeDelta = it.VolumeSurge - p.VolumeSurge
		}
		c.prev[it.Symbol] = radarPrevSnap{
			Score: it.TradeScore, Price: it.Price, VolumeSurge: it.VolumeSurge,
		}
	}
}
