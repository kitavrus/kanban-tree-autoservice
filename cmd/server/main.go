package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/features/board"
	"github.com/kitavrus/kanban-tree/internal/filestore"
	"github.com/kitavrus/kanban-tree/internal/observability"
	"github.com/kitavrus/kanban-tree/internal/server"
)

// @title           Kanban-tree API
// @version         1.0
// @description     JSON API (CLAUDE.md §API-1: kebab-URL, snake_case JSON, списки {items,total,limit,offset}).
// @description     Аннотированы эндпоинты закупок по дефициту и кабинета поставщика (purchase-orders).
// @BasePath        /api/v1
// @securityDefinitions.apikey CookieSession
// @in              cookie
// @name            sid
func main() {
	// --check-config ОБЯЗАТЕЛЕН (контракт infra-bootstrap): образ проверяет env
	// своим кодом и выходит 0/1, не поднимая БД/Redis/портов.
	checkConfig := flag.Bool("check-config", false, "validate env and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	if *checkConfig {
		os.Exit(0)
	}

	logger := observability.NewLogger(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracer, err := observability.InitTracer(ctx, "kanban-tree", cfg.OTLPEndpoint)
	if err != nil {
		logger.Error("otel init", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("pgx pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()

	app, err := server.New(cfg, logger, pool, rdb)
	if err != nil {
		logger.Error("server init", "err", err)
		os.Exit(1)
	}

	// Сборщик файлов-сирот UPLOADS_DIR — фоновый тикер в процессе сервера (не отдельная
	// команда): каталог и БД уже здесь, отдельный планировщик (cron) не нужен, первый проход
	// при старте ловит сирот сразу после аварийного рестарта. Идемпотентен — два инстанса
	// (blue-green) на одном каталоге не конфликтуют.
	if cfg.UploadsGC.Interval > 0 {
		files, err := filestore.NewLocal(cfg.UploadsDir)
		if err != nil {
			logger.Error("uploads gc init", "err", err)
			os.Exit(1)
		}
		go filestore.RunOrphanCollector(ctx, files, board.NewAttachmentRepo(pool),
			cfg.UploadsGC.Interval, cfg.UploadsGC.MinAge, logger)
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutdown")
		_ = app.Shutdown()
	}()

	logger.Info("listening", "port", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		logger.Error("listen", "err", err)
		os.Exit(1)
	}
}
