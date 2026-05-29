package aegis

// Secrets are configured via Encore: encore secret set --dev <Name>
var secrets struct {
	BinanceAPIKey       string
	BinanceAPISecret    string
	BinanceUseTestnet   string
	CoinglassAPIKey     string
	TelegramBotToken    string
	TelegramChatID      string
	AegisTradingEnabled string
	AegisEnv            string
}

func tradingEnabled() bool {
	return secrets.AegisTradingEnabled == "true" || secrets.AegisTradingEnabled == "1"
}

func useTestnet() bool {
	return secrets.BinanceUseTestnet == "true" || secrets.BinanceUseTestnet == "1"
}
