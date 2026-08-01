package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/goropikari/go-project/authz"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	address := flag.String("address", ":8080", "HTTP listen address")
	dsn := flag.String("dsn", os.Getenv("AUTHZ_DATABASE_URL"), "PostgreSQL connection string")

	flag.Parse()

	if *dsn == "" {
		return fmt.Errorf("AUTHZ_DATABASE_URL or -dsn is required")
	}

	store, err := authz.NewPostgresStore(*dsn)
	if err != nil {
		return err
	}

	defer func() { _ = store.Close() }()

	if os.Getenv("AUTHZ_SEED_DEMO") == "true" {
		if err := store.SeedDemo(); err != nil {
			return err
		}
	}

	log.Printf("authorization service listening on %s", *address)

	server := &http.Server{Addr: *address, Handler: authz.NewHandler(store), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}

	return server.ListenAndServe()
}
