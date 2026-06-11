package config

import "sync"

const PaperTakerFeeBps = 4.0 // matches backtest assumption

var (
	paperMu     sync.RWMutex
	paperModeOn bool
)

func SetPaperMode(on bool) {
	paperMu.Lock()
	paperModeOn = on
	paperMu.Unlock()
}

func IsPaperMode() bool {
	paperMu.RLock()
	defer paperMu.RUnlock()
	return paperModeOn
}
