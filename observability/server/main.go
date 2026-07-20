package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "api-server"
	}
	logPath := os.Getenv("LOG_FILE")
	if logPath == "" {
		logPath = "/logs/app.jsonl"
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, logFile), &slog.HandlerOptions{Level: slog.LevelInfo})).With("service.name", serviceName)
	slog.SetDefault(logger)

	interval := 2 * time.Second
	if raw := os.Getenv("LOG_INTERVAL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			interval = d
		}
	}

	go emitDemoLogs(interval)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("request completed", "method", r.Method, "path", r.URL.Path, "status", 200, "request_id", requestID())
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "observe-log-server", "status": "ok"})
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	http.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		logger.Error("simulated application error", "error_code", "DEMO-500", "request_id", requestID())
		http.Error(w, "simulated error", http.StatusInternalServerError)
	})
	logger.Info("server started", "listen", ":8080", "interval", interval.String())
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.Error("server stopped", "error", err)
	}
}

func emitDemoLogs(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		status := []int{200, 200, 200, 404, 500}[rand.IntN(5)]
		level := slog.LevelInfo
		message := "demo request completed"
		if status >= 500 {
			level, message = slog.LevelError, "demo request failed"
		}
		if status == 404 {
			level, message = slog.LevelWarn, "demo route not found"
		}
		attrs := []any{"method", "GET", "path", "/api/items/" + strconv.Itoa(rand.IntN(10)+1), "status", status, "latency_ms", rand.IntN(400) + 10, "request_id", requestID(), "env", "comparison"}
		slog.Default().Log(context.Background(), level, message, attrs...)
	}
}

func requestID() string { return strconv.FormatInt(time.Now().UnixNano(), 36) }
