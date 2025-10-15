package link

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"

	"shotr/db"
	"shotr/workers"
)

// enqueueClick sends a click to the worker if possible.
// If the worker is full or not configured, falls back to direct writes.
func (l *Link) enqueueClick(ctx context.Context, slug string) {
	ev := workers.ClickEvent{Slug: slug, Time: time.Now()}

	if l.Worker != nil {
		if l.Worker.Enqueue(ev) {
			return
		}
		// fallback: worker full
		l.writeClickFallback(ctx, slug, "worker full")
		return
	}

	// no worker at all
	l.writeClickFallback(ctx, slug, "no worker configured")
}

func (l *Link) writeClickFallback(ctx context.Context, slug, reason string) {
	if err := l.Q.AddClick(ctx, db.AddClickParams{
		Clicks: sql.NullInt64{Int64: 1, Valid: true},
		Slug:   slug,
	}); err != nil {
		l.Log.Debug("fallback AddClick failed", zap.String("slug", slug), zap.Error(err))
	}
	if err := l.Q.SaveDailyClicks(ctx, db.SaveDailyClicksParams{
		Slug:   slug,
		Clicks: sql.NullInt64{Int64: 1, Valid: true},
	}); err != nil {
		l.Log.Debug("fallback SaveDailyClicks failed", zap.String("slug", slug), zap.Error(err))
	}
	l.Log.Debug("click worker fallback sync increment", zap.String("slug", slug), zap.String("reason", reason))
}