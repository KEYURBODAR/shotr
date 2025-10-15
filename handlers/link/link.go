package link

import (
	"database/sql"
	"net/http"

	lru "github.com/hashicorp/golang-lru"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"shotr/db"
	h "shotr/helpers"
	"shotr/workers"
)

// Link handler contains dependencies for link endpoints.
type Link struct {
	Q        *db.Queries
	Log      *zap.Logger
	BaseHost string
	Worker   *workers.ClickWorker
	Cache    *lru.Cache
}

func New(q *db.Queries, log *zap.Logger, baseHost string, cw *workers.ClickWorker, cache *lru.Cache) *Link {
	return &Link{
		Q:        q,
		Log:      log,
		BaseHost: baseHost,
		Worker:   cw,
		Cache:    cache,
	}
}

// POST /api/v1/links
func (l *Link) Create(c echo.Context) error {
	var req struct {
		URL string `json:"url" validate:"required,url"`
	}
	if err := h.BindAndValidate(c, &req); err != nil {
		return err
	}

	ctx := c.Request().Context()
	link, err := h.TryInsertWithRetry(ctx, l.Q, req.URL, 5, l.Log)
	if err != nil {
		l.Log.Error("failed to create short link", zap.Error(err))
		return h.JSONError(c, http.StatusInternalServerError, "couldn't create short link")
	}

	short := h.BuildShortURL(c, l.BaseHost, link.Slug)
	c.Response().Header().Set("Location", short)

	return h.JSONSuccess(c, http.StatusCreated, map[string]any{
		"slug":      link.Slug,
		"short_url": short,
		"id":        link.ID,
	}, "")
}

// GET /:slug  and HEAD
func (l *Link) Redirect(c echo.Context) error {
	slug := c.Param("slug")
	if slug == "" {
		return h.JSONError(c, http.StatusBadRequest, "missing slug")
	}

	ctx := c.Request().Context()
	url, _, err := l.resolveURL(ctx, slug)
	if err == sql.ErrNoRows {
		return h.JSONError(c, http.StatusNotFound, "not found")
	}
	if err != nil {
		l.Log.Error("db lookup failed", zap.Error(err))
		return h.JSONError(c, http.StatusInternalServerError, "db error")
	}

	l.enqueueClick(ctx, slug)
	return c.Redirect(http.StatusFound, url)
}

// GET /api/v1/links/:slug/stats
func (l *Link) Stats(c echo.Context) error {
	slug := c.Param("slug")
	if slug == "" {
		return h.JSONError(c, http.StatusBadRequest, "missing slug")
	}

	ctx := c.Request().Context()
	linkRow, err := l.Q.GetLinkStats(ctx, slug)
	if err == sql.ErrNoRows {
		return h.JSONError(c, http.StatusNotFound, "not found")
	}
	if err != nil {
		l.Log.Error("failed to fetch link stats", zap.Error(err))
		return h.JSONError(c, http.StatusInternalServerError, "db error")
	}

	dailyRows, err := l.Q.GetDailyClicks(ctx, slug)
	if err != nil {
		l.Log.Error("failed to fetch daily clicks", zap.Error(err))
		return h.JSONError(c, http.StatusInternalServerError, "db error")
	}

	daily := make([]map[string]any, 0, len(dailyRows))
	for _, r := range dailyRows {
		clicks := int64(0)
		if r.Clicks.Valid {
			clicks = r.Clicks.Int64
		}
		daily = append(daily, map[string]any{
			"day":    r.Day.Format("2006-01-02"),
			"clicks": clicks,
		})
	}

	total := int64(0)
	if linkRow.Clicks.Valid {
		total = linkRow.Clicks.Int64
	}

	return h.JSONSuccess(c, http.StatusOK, map[string]any{
		"slug":  linkRow.Slug,
		"total": total,
		"daily": daily,
		"url":   linkRow.Url,
	}, "")
}