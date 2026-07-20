package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goropikari/study_memo/simple_webapp/internal/application"
	"github.com/goropikari/study_memo/simple_webapp/internal/infrastructure"
	"github.com/goropikari/study_memo/simple_webapp/internal/presentation"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := envOrDefault("ADDR", ":8080")
	databasePath := envOrDefault("DATABASE_PATH", "study_memo.db")

	dbManager, err := infrastructure.NewDBManager(databasePath)
	if err != nil {
		return err
	}
	defer dbManager.Close()

	userRepo := infrastructure.NewSQLiteUserRepository(dbManager.DB())
	memoRepo := infrastructure.NewSQLiteMemoRepository(dbManager.DB())
	authService := application.NewAuthService(userRepo)
	memoService := application.NewMemoService(memoRepo)
	sessionStore := infrastructure.NewMemorySessionStore()
	sessionManager := presentation.NewSessionManager(sessionStore, presentation.SessionConfig{
		CookieName: "study_session",
		MaxAge:     24 * time.Hour,
	})
	handlers := presentation.NewHandlers(authService, memoService, sessionManager)

	server := &http.Server{
		Addr:              addr,
		Handler:           presentation.NewRouter(handlers),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("server listening on %s", addr)
		errCh <- server.ListenAndServe()
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdownCh)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownCh:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
