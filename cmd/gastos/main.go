package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"gastos/internal/api"
	"gastos/internal/config"
	"gastos/internal/db"
	"gastos/internal/events"
	"gastos/internal/recurrence"
	"gastos/web"
)

var version = "dev"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("encerrando", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	d, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer d.Close()

	bus := events.New()
	mat := recurrence.NewMaterializer(d, bus, cfg.Location, log)
	srv := api.New(d, cfg, bus, mat, log)

	go mat.Loop(ctx, time.Hour) // runs once right away
	go srv.Housekeeping(ctx, time.Hour)

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(web.Dist()),
		ReadHeaderTimeout: 10 * time.Second,
		// no WriteTimeout: SSE connections stay open indefinitely
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpSrv.Shutdown(shutdownCtx)
	}()

	log.Info("ouvindo", "addr", cfg.Addr, "version", version, "db", cfg.DBPath)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
