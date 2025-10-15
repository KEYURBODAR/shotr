package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"                // loads .env automatically if present
	_ "github.com/mattn/go-sqlite3"                      // local fallback driver (sqlite file)
	_ "github.com/tursodatabase/libsql-client-go/libsql" // libSQL (Turso) driver

	"go.uber.org/zap"

	"shotr/config"
	"shotr/db"
	"shotr/workers"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var logger *zap.Logger
	if cfg.LogLevel == "debug" || cfg.AppEnv == "development" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()

	if err := os.MkdirAll("data", 0o755); err != nil {
		logger.Fatal("create data dir", zap.Error(err))
	}

	var dbConn *sql.DB
	if cfg.DatabaseURL != "" {
		logger.Info("using libsql (Turso) DB", zap.String("url", cfg.DatabaseURL))
		dbConn, err = sql.Open("libsql", cfg.DatabaseURL)
		if err != nil {
			logger.Fatal("open libsql", zap.Error(err))
		}
	} else {
		logger.Info("using local sqlite file", zap.String("path", cfg.DatabasePath))
		dbConn, err = sql.Open("sqlite3", cfg.DatabasePath)
		if err != nil {
			logger.Fatal("open sqlite", zap.Error(err))
		}
		_, _ = dbConn.Exec("PRAGMA journal_mode=WAL;")
		_, _ = dbConn.Exec("PRAGMA synchronous=NORMAL;")
	}

	dbConn.SetMaxOpenConns(1)
	dbConn.SetMaxIdleConns(1)
	defer dbConn.Close()

	q := db.New(dbConn)

	cw := workers.NewClickWorker(dbConn, q, logger, 400, 250*time.Millisecond, 8192)
	cw.Start()
	defer cw.Stop()

	srv := NewServer(dbConn, logger, q, cfg.BaseHost)
	srv.ClickWorkers = cw

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Info("starting server", zap.String("address", addr))
	if err := srv.Start(addr); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}
