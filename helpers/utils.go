package helpers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"shotr/db"

	"github.com/avast/retry-go"
	"github.com/labstack/echo/v4"
	"github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func TryInsertWithRetry(ctx context.Context, q *db.Queries, url string, maxRetries int, log *zap.Logger) (db.Link, error) {
	var created db.Link
	params := db.AddLinkParams{
		Url:  url,
		User: sql.NullString{Valid: false},
	}

	operation := func() error {
		slug, err := New()
		if err != nil {
			return retry.Unrecoverable(err)
		}
		params.Slug = slug

		link, err := q.AddLink(ctx, params)
		if err == nil {
			created = link
			return nil
		}

		if isUniqueConstraint(err) {
			return err
		}

		return retry.Unrecoverable(err)
	}

	err := retry.Do(
		operation,
		retry.Attempts(uint(maxRetries)),
		retry.Delay(50*time.Millisecond),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			log.Warn("retrying slug insert", zap.Uint("attempt", n+1), zap.Error(err))
		}),
	)

	return created, err
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}

	var se sqlite3.Error
	if errors.As(err, &se) {
		if se.ExtendedCode == sqlite3.ErrConstraintUnique || se.Code == sqlite3.ErrConstraint {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "constraint failed")
}

func BuildShortURL(c echo.Context, baseHost, slug string) string {
	if baseHost != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(baseHost, "/"), slug)
	}

	req := c.Request()
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	
	return fmt.Sprintf("%s://%s/%s", scheme, req.Host, slug)
}

func IncClicks(ctx context.Context, q *db.Queries, n int64, slug string, log *zap.Logger) {
	go func() {
		err := q.AddClick(ctx, db.AddClickParams{
			Clicks: sql.NullInt64{Int64: n, Valid: true},
			Slug: slug,
		})
		if err != nil && log != nil {
			log.Warn("failed to increment clicks", zap.Error(err), zap.String("slug", slug))
		}

		if err := q.SaveDailyClicks(ctx, db.SaveDailyClicksParams{
			Slug: slug,
			Clicks: sql.NullInt64{
				Int64: n,
				Valid: true,
			},
		}); err != nil && log != nil {
			log.Warn("failed to increment daily clicks", zap.Error(err), zap.String("slug", slug))
		}
	}()
} 