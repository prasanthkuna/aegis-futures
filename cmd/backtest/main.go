package main



import (

	"context"

	"flag"

	"fmt"

	"os"

	"strings"

	"time"



	"encore.app/backtest"

)



func main() {

	days := flag.Int("days", 90, "history days (falls back to 60 if cache missing)")

	suite := flag.Bool("suite", false, "run BT-1..BT-5 suite")

	pro := flag.Bool("pro", false, "pro factory v2: promotion funnel (strict live gates)")
	discover := flag.String("discover", "", "catalog profitable strategies: lean, coarse, or enriched")
	fetchContext := flag.Bool("fetch-context", false, "download Binance OI/funding/flow context for top 30 symbols")
	proof := flag.Bool("proof", false, "run focused profit proof grid (~32 configs)")
	fastCheck := flag.Bool("fastcheck", false, "quick u30 check: baseline vs BB_STRETCH_REVERT (5 runs)")
	scan := flag.Bool("scan", false, "strategy scan: ~18 hypotheses on u30/60d")
	btcScan := flag.Bool("btcscan", false, "run all playbook strategies on one symbol (default BTCUSDT)")
	btcLab := flag.Bool("btclab", false, "custom BTC strategies designed from scratch (not Aegis playbooks)")
	btcRefine := flag.Bool("btcrefine", false, "retest proven BTC strategies on 15m/1h/4h with 5x leverage sizing")
	btcProd := flag.Bool("btcprod", false, "liberal 1h BTC prod book: loose max trades, 5x $5 risk")
	coreValidate := flag.Bool("corevalidate", false, "validate proven BTC/SOL/ETH strategies on 90+180d")
	solS4Check := flag.Bool("sols4check", false, "validate SOL S4 squeeze liberal on 90+180d")
	symbol := flag.String("symbol", "BTCUSDT", "symbol for -btcscan (e.g. BTCUSDT, ETHUSDT)")
	hunt := flag.Bool("hunt", false, "profit hunt: rank all configs by net PnL on cached data")

	legacy := flag.Bool("legacy", false, "legacy brute-force factory grid")

	cachedOnly := flag.Bool("cached-only", true, "use cached klines only (no downloads)")

	top := flag.Int("top", 10, "number of top strategies to print")

	name := flag.String("name", "", "single run name (optional)")

	flag.Parse()



	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)

	defer cancel()



	store := backtest.NewDataStore("")

	runner := backtest.NewRunner(store)



	if *solS4Check && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "SOL S4 liberal validation (90+180d)...\n")
		start := time.Now()
		var all []backtest.CoreProvenRow
		for _, d := range []int{90, 180} {
			fmt.Fprintf(os.Stderr, "--- %dd ---\n", d)
			rows, elapsed, err := backtest.RunSOLS4Proven(ctx, runner, d)
			if err != nil {
				fmt.Fprintf(os.Stderr, "sols4check %dd failed: %v\n", d, err)
				os.Exit(1)
			}
			all = append(all, rows...)
			fmt.Fprintf(os.Stderr, "  %dd done in %s\n", d, elapsed.Round(time.Second))
		}
		out := backtest.FormatSOLS4Proven(all, time.Since(start))
		fmt.Print(out)
		_ = os.WriteFile("backtest/sol-s4-proven-90-180.txt", []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "Saved backtest/sol-s4-proven-90-180.txt\n")
		return
	}

	if *coreValidate && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "Core proven validation (BTC/SOL/ETH)...\n")
		horizons := []int{90, 180}
		var all []backtest.CoreProvenRow
		start := time.Now()
		byDays := map[int][]backtest.CoreProvenRow{}
		for _, d := range horizons {
			fmt.Fprintf(os.Stderr, "\n--- %dd (fetch if missing) ---\n", d)
			rows, elapsed, err := backtest.RunCoreProven(ctx, runner, d)
			if err != nil {
				fmt.Fprintf(os.Stderr, "corevalidate %dd failed: %v\n", d, err)
				os.Exit(1)
			}
			byDays[d] = rows
			all = append(all, rows...)
			fmt.Fprintf(os.Stderr, "  %dd done in %s\n", d, elapsed.Round(time.Second))
		}
		out := backtest.FormatCoreProvenMulti(byDays, time.Since(start))
		fmt.Print(out)
		_ = os.WriteFile("backtest/core-proven-90-180.txt", []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved backtest/core-proven-90-180.txt\n")
		return
	}

	if *btcProd && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "BTC prod liberal 1h on %s...\n", *symbol)
		d := store.BestCacheDays(*days)
		rows, combo, elapsed, err := backtest.RunBTCProdLiberal(ctx, runner, *symbol, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "btcprod failed: %v\n", err)
			os.Exit(1)
		}
		out := backtest.FormatBTCProd(*symbol, rows, combo, d, elapsed)
		fmt.Print(out)
		path := fmt.Sprintf("backtest/btc-prod-%s.txt", strings.ToLower(*symbol))
		_ = os.WriteFile(path, []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved %s | %s\n", path, elapsed.Round(time.Second))
		return
	}

	if *btcRefine && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "BTC refine (15m/1h/4h + leverage) on %s...\n", *symbol)
		d := store.BestCacheDays(*days)
		rows, elapsed, err := backtest.RunBTCRefine(ctx, runner, *symbol, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "btcrefine failed: %v\n", err)
			os.Exit(1)
		}
		out := backtest.FormatBTCRefine(*symbol, rows, d, elapsed)
		fmt.Print(out)
		path := fmt.Sprintf("backtest/btc-refine-%s.txt", strings.ToLower(*symbol))
		_ = os.WriteFile(path, []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved %s | %s\n", path, elapsed.Round(time.Second))
		return
	}

	if *btcLab && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "BTC lab (custom strategies) on %s...\n", *symbol)
		d := store.BestCacheDays(*days)
		results, elapsed, err := backtest.RunBTCLab(ctx, runner, *symbol, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "btclab failed: %v\n", err)
			os.Exit(1)
		}
		out := backtest.FormatBTCLab(*symbol, results, d, elapsed)
		fmt.Print(out)
		path := fmt.Sprintf("backtest/btc-lab-%s.txt", strings.ToLower(*symbol))
		_ = os.WriteFile(path, []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved %s | %s\n", path, elapsed.Round(time.Second))
		return
	}

	if *btcScan && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "Symbol scan %s...\n", *symbol)
		d := store.BestCacheDays(*days)
		rows, bars, elapsed, err := backtest.RunSymbolScan(ctx, runner, *symbol, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "btcscan failed: %v\n", err)
			os.Exit(1)
		}
		out := backtest.FormatSymbolScan(*symbol, rows, d, bars, elapsed)
		fmt.Print(out)
		path := fmt.Sprintf("backtest/scan-%s.txt", strings.ToLower(*symbol))
		_ = os.WriteFile(path, []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved %s | %s\n", path, elapsed.Round(time.Second))
		return
	}

	if *scan && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "Strategy scan...\n")
		out, elapsed, err := backtest.RunStrategyScanWithAttribution(ctx, runner, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(out)
		_ = os.WriteFile("backtest/strategy-scan.txt", []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved backtest/strategy-scan.txt | %s\n", elapsed.Round(time.Second))
		return
	}

	if *fastCheck && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "Fast check...\n")
		d := store.BestCacheDays(*days)
		rows, elapsed, err := backtest.RunFastCheck(ctx, runner, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fastcheck failed: %v\n", err)
			os.Exit(1)
		}
		out := backtest.FormatFastCheck(rows, d, elapsed)
		fmt.Print(out)
		_ = os.WriteFile("backtest/fast-check.txt", []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved backtest/fast-check.txt | %s\n", elapsed.Round(time.Second))
		return
	}

	if *hunt && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "Profit hunt...\n")
		rep, err := backtest.RunProfitHunt(ctx, runner, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hunt failed: %v\n", err)
			os.Exit(1)
		}
		out := backtest.FormatProfitHuntReport(rep)
		fmt.Print(out)
		_ = os.WriteFile("backtest/profit-hunt.txt", []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved backtest/profit-hunt.txt | %d profitable | %s\n",
			rep.Profitable, rep.Elapsed.Round(time.Second))
		return
	}

	if *proof && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "Profit proof run...\n")
		rep, err := backtest.RunProfitProof(ctx, runner, *days)
		if err != nil {
			fmt.Fprintf(os.Stderr, "proof failed: %v\n", err)
			os.Exit(1)
		}
		out := backtest.FormatProofReport(rep)
		fmt.Print(out)
		_ = os.WriteFile("backtest/profit-proof.txt", []byte(out), 0o644)
		fmt.Fprintf(os.Stderr, "\nSaved backtest/profit-proof.txt | Done in %s\n", rep.Elapsed.Round(time.Second))
		return
	}

	if *fetchContext {
		d := store.BestCacheDays(*days)
		syms := store.TopCachedSymbols(d, 30)
		fmt.Fprintf(os.Stderr, "Fetching Binance context for %d symbols (%dd)...\n", len(syms), d)
		n, err := store.FetchContextBatch(ctx, syms, d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch-context failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Cached context for %d/%d symbols\n", n, len(syms))
		return
	}

	if *discover != "" && !*legacy && !*suite && *name == "" {
		fmt.Fprintf(os.Stderr, "Discovery catalog (%s grid)...\n", *discover)
		rep, err := backtest.RunDiscovery(ctx, runner, *days, *discover)
		if err != nil {
			fmt.Fprintf(os.Stderr, "discovery failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(backtest.FormatDiscoveryReport(rep))
		fmt.Fprintf(os.Stderr, "\nDone in %s\n", rep.Elapsed.Round(time.Second))
		return
	}

	if *pro && !*legacy && !*suite && *name == "" {

		actualDays := store.BestCacheDays(*days)

		fmt.Fprintf(os.Stderr, "Pro factory v2 (%dd cache, %d requested)...\n", actualDays, *days)

		rep, err := backtest.RunProFactory(ctx, runner, *days, *top)

		if err != nil {

			fmt.Fprintf(os.Stderr, "pro factory failed: %v\n", err)

			os.Exit(1)

		}

		fmt.Print(backtest.FormatProFactoryReport(rep))

		fmt.Fprintf(os.Stderr, "\nDone in %s\n", rep.Elapsed.Round(time.Second))

		return

	}



	if *legacy {

		fmt.Fprintf(os.Stderr, "Legacy factory: 1344 configs (%d days, cached=%v)...\n", *days, *cachedOnly)

		start := time.Now()

		ranked, err := backtest.RunFactory(ctx, runner, *days, *top)

		if err != nil {

			fmt.Fprintf(os.Stderr, "factory failed: %v\n", err)

			os.Exit(1)

		}

		fmt.Print(backtest.FormatTopStrategies(ranked, false))

		fmt.Print(backtest.FormatStrategyPlaybook(ranked))

		fmt.Fprintf(os.Stderr, "\nFactory completed in %s\n", time.Since(start).Round(time.Second))

		return

	}



	if *suite {

		mode := "downloading/caching klines"

		if *cachedOnly {

			mode = "cached klines only from " + store.CacheDir

		}

		fmt.Fprintf(os.Stderr, "Running backtest suite (%d days) — %s...\n", *days, mode)

		start := time.Now()

		rep, err := backtest.RunSuite(ctx, runner, *days, *cachedOnly)

		if err != nil {

			fmt.Fprintf(os.Stderr, "suite failed: %v\n", err)

			os.Exit(1)

		}

		fmt.Print(backtest.FormatSuiteTable(rep))

		fmt.Print(backtest.FormatBT2SessionBreakdown(rep))

		fmt.Fprintf(os.Stderr, "\nCompleted in %s\n", time.Since(start).Round(time.Second))

		return

	}



	cfg := backtest.RunConfig{Name: "custom", Days: *days, UniverseTopN: 30}

	if *name != "" {

		cfg.Name = *name

	}

	res, err := runner.Run(ctx, cfg)

	if err != nil {

		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)

		os.Exit(1)

	}

	fmt.Printf("%+v\n", res)

}


