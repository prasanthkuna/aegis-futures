package binanceex

// Mainnet is production default. Testnet is a minimal URL switch for one smoke test.
type Network struct {
	RESTBase string
	WSBase   string
}

func NetworkForTestnet(testnet bool) Network {
	if testnet {
		return Network{
			RESTBase: "https://testnet.binancefuture.com",
			WSBase:   "wss://fstream.binancefuture.com",
		}
	}
	return Network{
		RESTBase: "https://fapi.binance.com",
		WSBase:   "wss://fstream.binance.com",
	}
}
