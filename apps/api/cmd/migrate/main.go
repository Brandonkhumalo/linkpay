package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}
	files, err := filepath.Glob(filepath.Join(dir, "[0-9]*.sql"))
	if err != nil || len(files) == 0 {
		log.Fatalf("read migrations: %v (files: %d)", err, len(files))
	}
	sort.Strings(files)
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	for _, file := range files {
		version := strings.TrimSuffix(filepath.Base(file), ".sql")
		sql, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			log.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(114271991)`); err == nil {
			_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`)
		}
		var applied bool
		if err == nil {
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied)
		}
		if err == nil && !applied {
			_, err = tx.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol)
		}
		if err == nil && !applied {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err != nil {
			log.Fatalf("migration %s: %v", version, err)
		}
		if !applied {
			log.Printf("migration %s applied", version)
		}
	}
}
