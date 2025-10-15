package workers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"go.uber.org/zap"

	"shotr/db"
)

// ClickEvent represents one click for a slug.
type ClickEvent struct {
	Slug string
	Time time.Time
}

// ClickWorker batches click events and writes them to DB using single upserts.
type ClickWorker struct {
	db            *sql.DB       // raw DB handle for multi-row upserts
	q             *db.Queries   // keep if you want to call other generated queries
	log           *zap.Logger
	in            chan ClickEvent
	batchSize     int
	flushInterval time.Duration
	closed        chan struct{}
}

// NewClickWorker creates the worker. Requires sqlDB (the *sql.DB you opened).
func NewClickWorker(sqlDB *sql.DB, q *db.Queries, log *zap.Logger, batchSize int, flushInterval time.Duration, buffer int) *ClickWorker {
	return &ClickWorker{
		db:            sqlDB,
		q:             q,
		log:           log,
		in:            make(chan ClickEvent, buffer),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		closed:        make(chan struct{}),
	}
}

func (w *ClickWorker) Start() { go w.loop() }

func (w *ClickWorker) Stop() {
	close(w.in)
	<-w.closed
}

func (w *ClickWorker) Enqueue(ev ClickEvent) bool {
	select {
	case w.in <- ev:
		return true
	default:
		return false
	}
}

// buildUpsertLinks builds a multi-row upsert for links table.
// returns query string and args slice.
func buildUpsertLinks(rows map[string]int64) (string, []interface{}) {
	n := len(rows)
	// VALUES (?, ?), (?, ?) ...
	v := make([]string, 0, n)
	args := make([]interface{}, 0, n*2)
	for slug, cnt := range rows {
		v = append(v, "(?, ?)")
		args = append(args, slug, cnt)
	}
	q := fmt.Sprintf(
		"INSERT INTO links (slug, clicks) VALUES %s ON CONFLICT(slug) DO UPDATE SET clicks = clicks + excluded.clicks;",
		strings.Join(v, ","),
	)
	return q, args
}

// buildUpsertDaily builds multi-row upsert for daily_clicks (uses date('now')).
func buildUpsertDaily(rows map[string]int64) (string, []interface{}) {
	n := len(rows)
	v := make([]string, 0, n)
	args := make([]interface{}, 0, n*2)
	for slug, cnt := range rows {
		// VALUES (?, date('now'), ?)
		v = append(v, "(?, date('now'), ?)")
		args = append(args, slug, cnt)
	}
	q := fmt.Sprintf(
		"INSERT INTO daily_clicks (slug, day, clicks) VALUES %s ON CONFLICT(slug, day) DO UPDATE SET clicks = clicks + excluded.clicks;",
		strings.Join(v, ","),
	)
	return q, args
}

func (w *ClickWorker) loop() {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	defer close(w.closed)

	counts := make(map[string]int64)
	total := 0

	flush := func() {
		if total == 0 {
			return
		}
		toFlush := counts
		counts = make(map[string]int64)
		total = 0

		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()

		// Build SQL and args
		linksQ, linksArgs := buildUpsertLinks(toFlush)
		dailyQ, dailyArgs := buildUpsertDaily(toFlush)

		// Perform both upserts inside one transaction with retries
		err := retry.Do(
			func() error {
				tx, err := w.db.BeginTx(ctx, nil)
				if err != nil {
					return err
				}
				// execute links upsert
				if _, err := tx.ExecContext(ctx, linksQ, linksArgs...); err != nil {
					_ = tx.Rollback()
					return err
				}
				// execute daily upsert
				if _, err := tx.ExecContext(ctx, dailyQ, dailyArgs...); err != nil {
					_ = tx.Rollback()
					return err
				}
				if err := tx.Commit(); err != nil {
					_ = tx.Rollback()
					return err
				}
				return nil
			},
			retry.Attempts(3),
			retry.Delay(125*time.Millisecond),
			retry.DelayType(retry.BackOffDelay),
			retry.OnRetry(func(n uint, err error) {
				w.log.Warn("retrying upsert batch", zap.Uint("attempt", n+1), zap.Error(err))
			}),
		)

		if err != nil {
			// If this fails repeatedly, fallback to per-slug updates to try to preserve counts.
			w.log.Error("multi-upsert failed; attempting per-slug fallback", zap.Int("unique_slugs", len(toFlush)), zap.Error(err))
			w.perSlugFallback(ctx, toFlush)
			return
		}
		w.log.Debug("multi-upsert flushed", zap.Int("unique_slugs", len(toFlush)))
	}

	for {
		select {
		case ev, ok := <-w.in:
			if !ok {
				flush()
				return
			}
			counts[ev.Slug]++
			total++
			if total >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			if total > 0 {
				flush()
			}
		}
	}
}

// perSlugFallback tries to write each slug individually (less efficient) if multi-upsert fails.
func (w *ClickWorker) perSlugFallback(ctx context.Context, rows map[string]int64) {
	for slug, cnt := range rows {
		if cnt <= 0 {
			continue
		}
		// try add click
		if err := w.q.AddClick(ctx, db.AddClickParams{
			Clicks: sql.NullInt64{Int64: cnt, Valid: true},
			Slug:   slug,
		}); err != nil {
			w.log.Error("fallback AddClick failed", zap.String("slug", slug), zap.Int64("count", cnt), zap.Error(err))
		}
		// try daily
		if err := w.q.SaveDailyClicks(ctx, db.SaveDailyClicksParams{
			Slug:   slug,
			Clicks: sql.NullInt64{Int64: cnt, Valid: true},
		}); err != nil {
			w.log.Error("fallback SaveDailyClicks failed", zap.String("slug", slug), zap.Int64("count", cnt), zap.Error(err))
		}
	}
}