package aegis

// Required Encore Cloud secrets. Set via Encore dashboard or:
// encore secret set --type prod,dev <Name>
var secrets struct {
	BinanceAPIKey       string
	BinanceAPISecret    string
	BinanceUseTestnet   string
	AegisTradingEnabled string
	AegisEnv            string
}

func binanceAPIKey() string       { return secrets.BinanceAPIKey }
func binanceAPISecret() string    { return secrets.BinanceAPISecret }
func binanceUseTestnetRaw() string { return secrets.BinanceUseTestnet }
func aegisTradingEnabledRaw() string {
	return secrets.AegisTradingEnabled
}
func aegisEnv() string { return secrets.AegisEnv }

func hasBinanceKeys() bool {
	return secrets.BinanceAPIKey != "" && secrets.BinanceAPISecret != ""
}

func tradingEnabled() bool {
	v := aegisTradingEnabledRaw()
	return v == "true" || v == "1"
}

func useTestnet() bool {
	v := binanceUseTestnetRaw()
	return v == "true" || v == "1"
}
