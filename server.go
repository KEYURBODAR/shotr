package main

import (
	"database/sql"
	"net/http"

	lru "github.com/hashicorp/golang-lru"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"shotr/db"
	link "shotr/handlers/link"
	"shotr/workers"
)

type Server struct {
	E        *echo.Echo
	DB       *sql.DB
	Q        *db.Queries
	Log      *zap.Logger
	BaseHost string
	ClickWorkers *workers.ClickWorker
	Cache *lru.Cache
}

func NewServer(dbConn *sql.DB, log *zap.Logger, q *db.Queries, baseHost string) *Server {
	e := echo.New()

	// essential middleware only
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())

	e.HideBanner = true
	e.HidePort = true

	s := &Server{
		E:        e,
		DB:       dbConn,
		Q:        q,
		Log:      log,
		BaseHost: baseHost,
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.E.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	link := link.New(s.Q, s.Log, s.BaseHost, s.ClickWorkers, s.Cache)

	s.E.POST("/api/v1/links", link.Create)
	s.E.GET("/:slug", link.Redirect)
	s.E.HEAD("/:slug", link.Redirect)
	s.E.GET("/api/v1/links/:slug/stats", link.Stats)
}

func (s *Server) Start(addr string) error {
	s.Log.Info("server starting", zap.String("addr", addr))
	return s.E.Start(addr)
}