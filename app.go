package main

import (
	"context"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"time"
)

var page = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="ru">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Счётчик посещений</title></head>
<body>
<h1>Hello from Helm!</h1>
<p>Количество посещений: <strong>{{.}}</strong></p>
<p>Обновите страницу, чтобы увеличить счётчик. Данные хранятся в PostgreSQL.</p>
</body>
</html>
`))

func newHandler(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		// Go routes HEAD to GET handlers; HEAD must not increment the counter.
		if r.Method == http.MethodHead {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		var count int64
		err := db.QueryRowContext(ctx, `INSERT INTO visits (id, count) VALUES (1, 1)
			ON CONFLICT (id) DO UPDATE SET count = visits.count + 1
			RETURNING count`).Scan(&count)
		if err != nil {
			slog.Error("increment counter", "error", err)
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := page.Execute(w, count); err != nil {
			slog.Error("render counter page", "error", err)
		}
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ready\n"))
	})
	return mux
}
