package ledger

import (
	"context"

	"encore.dev/storage/sqldb"
)

type Store struct {
	db *sqldb.Database
}

func New(db *sqldb.Database) *Store {
	return &Store{db: db}
}

func (s *Store) InsertRiskEvent(ctx context.Context, severity, typ, symbol, message, action string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO risk_events (severity, type, symbol, message, action_taken)
		VALUES ($1, $2, $3, $4, $5)
	`, severity, typ, symbol, message, action)
	return err
}

func (s *Store) InsertMissedTrade(ctx context.Context, symbol, side string, score float64, reason string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO missed_trades (symbol, side, trade_score, reason)
		VALUES ($1, $2, $3, $4)
	`, symbol, side, score, reason)
	return err
}

func (s *Store) InsertSetupScore(ctx context.Context, symbol string, score, vol, cvd, st, ctxC, depth, sess float64, decision, reason, side string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO setup_scores (
			symbol, trade_score, volume_component, cvd_component, structure_component,
			context_component, depth_component, session_component, decision, reason, side_hint
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, symbol, score, vol, cvd, st, ctxC, depth, sess, decision, reason, side)
	return err
}

func (s *Store) LogState(ctx context.Context, runID int64, from, to, reason string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO bot_state_log (bot_run_id, from_state, to_state, reason)
		VALUES ($1, $2, $3, $4)
	`, runID, from, to, reason)
	return err
}

func (s *Store) RealizedPnLSummary(ctx context.Context) (today, week, fees float64, err error) {
	err = s.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(net_pnl) FILTER (WHERE exit_time >= date_trunc('day', NOW() AT TIME ZONE 'UTC')), 0),
			COALESCE(SUM(net_pnl) FILTER (WHERE exit_time >= date_trunc('week', NOW() AT TIME ZONE 'UTC')), 0),
			COALESCE(SUM(fees), 0)
		FROM trades WHERE exit_time IS NOT NULL
	`).Scan(&today, &week, &fees)
	return
}
