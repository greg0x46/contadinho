// Command contadinho is the single-binary server: it opens (and migrates)
// the database — SQLite by default, or Postgres via -db's DSN — wires the
// HTTP API, runs the background sync worker, and serves the embedded
// frontend build, with no external runtime or deployment step required for
// the SQLite (default) setup.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
	"contadinho-go/internal/httpapi"
	"contadinho-go/internal/payables"
	"contadinho-go/internal/pluggy"
	"contadinho-go/internal/settings"
	"contadinho-go/internal/webui"
	"contadinho-go/internal/worker"
)

func main() {
	defaultDB := "contadinho.db"
	if envDB := os.Getenv("CONTADINHO_DB"); envDB != "" {
		defaultDB = envDB
	}
	dbPath := flag.String("db", defaultDB, "path to the SQLite database file, or a postgres://... / postgresql://... DSN to use Postgres instead (defaults to $CONTADINHO_DB if set)")
	addr := flag.String("addr", "localhost:4200", "address to listen on")
	flag.Parse()

	conn, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()

	// Same-person transfers synced before that label was mapped (see
	// categories.SourceCategoryMapping) still count as income in every total.
	// The fix travels with the binary, not with the schema, so it runs here
	// instead of as a migration: the mapping is Go code, and goose only has
	// SQL migrations to apply. BackfillAutomatic never touches a transaction
	// that already has a category, so running it on every start is a no-op
	// once the old rows are done.
	//
	// Logged rather than fatal: this pass fixes how old rows are *reported*,
	// so failing it is not a reason to refuse to start — and taking the
	// binary down would also take down the HTTP server that is the only way
	// to see what went wrong. The next start retries it.
	if applied, err := categories.BackfillAutomatic(context.Background(), conn, categories.SamePersonTransferLabel); err != nil {
		log.Printf("backfill %q categories failed, leaving those rows uncategorized: %v",
			categories.SamePersonTransferLabel, err)
	} else if applied > 0 {
		log.Printf("categorized %d %q transactions as transfers", applied, categories.SamePersonTransferLabel)
	}

	frontend, err := webui.DistFS()
	if err != nil {
		log.Fatalf("load embedded frontend: %v", err)
	}

	session := settings.NewSession()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx, conn, session, worker.Config{
		Pluggy:                pluggy.DefaultConfig(),
		OnTransactionUpserted: automation.NewTransactionHook(payables.UnlinkIfPresent),
	})

	handler := httpapi.NewServer(conn, frontend, session)
	log.Printf("listening on http://%s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
