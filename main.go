package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("barterswap: %v", err)
	}
}

func run() error {
	dsn := envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable")
	addr := ":" + envOr("PORT", "8080")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("ouverture de la base: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("connexion à la base: %w", err)
	}

	store := NewSQLStore(db)
	if err := store.Migrate(ctx); err != nil {
		return err
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           NewServer(NewApp(store)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("BarterSwap à l'écoute sur %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("arrêt en cours...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
