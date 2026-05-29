package aegis

// Required Encore Cloud secrets only (optional Telegram/CoinGlass removed from struct).
// Set via: encore secret set --type prod,dev <Name>
var secrets struct {
	BinanceAPIKey       string
	BinanceAPISecret    string
	BinanceUseTestnet   string
	AegisTradingEnabled string
	AegisEnv            string
}

func tradingEnabled() bool {
	return secrets.AegisTradingEnabled == "true" || secrets.AegisTradingEnabled == "1"
}

func useTestnet() bool {
	return secrets.BinanceUseTestnet == "true" || secrets.BinanceUseTestnet == "1"
}
