package backtest_test

import (
	"context"
	"os"
	"testing"
	"time"

	"encore.app/backtest"
)

func TestSuiteBT5(t *testing.T) {
	if os.Getenv("AEGIS_BACKTEST") != "1" {
		t.Skip("set AEGIS_BACKTEST=1 to run (downloads Binance klines)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	runner := backtest.NewRunner(backtest.NewDataStore("backtest/cache"))
	days := 60
	if v := os.Getenv("AEGIS_BT_DAYS"); v != "" {
		if n, err := time.ParseDuration(v + "h"); err == nil {
			days = int(n.Hours() / 24)
		}
	}
	rep, err := backtest.RunSuite(ctx, runner, days, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(backtest.FormatSuiteTable(rep))
	t.Log(backtest.FormatBT2SessionBreakdown(rep))
}
