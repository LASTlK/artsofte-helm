package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func databaseURL() (string, error) {
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		return "", fmt.Errorf("DB_PASSWORD is required")
	}
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(env("DB_HOST", "localhost"), env("DB_PORT", "5432")),
		User:   url.UserPassword(env("DB_USER", "counter"), password),
		Path:   "/" + env("DB_NAME", "counter"),
	}
	query := u.Query()
	query.Set("sslmode", env("DB_SSLMODE", "disable"))
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func openDatabase(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS visits (
		id SMALLINT PRIMARY KEY CHECK (id = 1),
		count BIGINT NOT NULL CHECK (count >= 0)
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize counter: %w", err)
	}
	return db, nil
}

func run() error {
	dsn, err := databaseURL()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	db, err := openDatabase(startupCtx, dsn)
	cancel()
	if err != nil {
		return err
	}
	defer db.Close()
	server := &http.Server{
		Addr: env("HTTP_ADDR", ":8080"), Handler: newHandler(db),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverError := make(chan error, 1)
	go func() {
		slog.Info("HTTP server started", "address", server.Addr)
		serverError <- server.ListenAndServe()
	}()
	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	}
}
