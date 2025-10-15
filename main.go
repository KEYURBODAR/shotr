// main.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload" // loads .env automatically if present
	_ "github.com/mattn/go-sqlite3"       // local fallback driver (sqlite file)
	_ "github.com/tursodatabase/libsql-client-go/libsql" // libSQL (Turso) driver

	"go.uber.org/zap"

	"shotr/config"
	"shotr/db"
	"shotr/workers"
)

func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Logger
	var logger *zap.Logger
	if cfg.LogLevel == "debug" || cfg.AppEnv == "development" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()

	// Ensure local data dir exists (used if running with local sqlite)
	if err := os.MkdirAll("data", 0o755); err != nil {
		logger.Fatal("create data dir", zap.Error(err))
	}

	// Open DB: prefer DATABASE_URL (libsql), fallback to local sqlite file.
	var dbConn *sql.DB
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		logger.Info("using libsql (Turso) DB", zap.String("url", dbURL))
		dbConn, err = sql.Open("libsql", dbURL)
		if err != nil {
			logger.Fatal("open libsql", zap.Error(err))
		}
	} else {
		logger.Info("using local sqlite file", zap.String("path", cfg.DatabasePath))
		dbConn, err = sql.Open("sqlite3", cfg.DatabasePath)
		if err != nil {
			logger.Fatal("open sqlite", zap.Error(err))
		}
		// sqlite-specific pragmas to improve write performance on local dev
		_, _ = dbConn.Exec("PRAGMA journal_mode=WAL;")
		_, _ = dbConn.Exec("PRAGMA synchronous=NORMAL;")
	}

	// DB pooling (safe defaults for our workload)
	dbConn.SetMaxOpenConns(1)
	dbConn.SetMaxIdleConns(1)
	defer dbConn.Close()

	// sqlc queries
	q := db.New(dbConn)

	// click worker (batching). tune these values if you see fallbacks in logs.
	cw := workers.NewClickWorker(dbConn, q, logger, 400, 250*time.Millisecond, 8192)
	cw.Start()
	defer cw.Stop()

	// start server
	srv := NewServer(dbConn, logger, q, cfg.BaseHost)
	srv.ClickWorkers = cw

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Info("starting server", zap.String("address", addr))
	if err := srv.Start(addr); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}