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
	"time"
	_ "time/tzdata" // static binaries have no system zoneinfo for CONTADINHO_SYNC_SCHEDULE zones

	"contadinho-go/internal/auth"
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
	if len(os.Args) > 1 && os.Args[1] == "auth" {
		if err := authCommand(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	defaultDB := "contadinho.db"
	if envDB := os.Getenv("CONTADINHO_DB"); envDB != "" {
		defaultDB = envDB
	}
	dbPath := flag.String("db", defaultDB, "path to the SQLite database file, or a postgres://... / postgresql://... DSN to use Postgres instead (defaults to $CONTADINHO_DB if set)")
	addr := flag.String("addr", "localhost:4200", "address to listen on")
	flag.Parse()

	config := auth.Config{PublicURL: os.Getenv("CONTADINHO_PUBLIC_URL")}
	if err := config.Validate(); err != nil {
		log.Fatal(err)
	}
	master, err := auth.LoadMasterKey()
	if err != nil {
		log.Fatal(err)
	}
	schedule, scheduled, err := worker.ParseSchedule(os.Getenv("CONTADINHO_SYNC_SCHEDULE"))
	if err != nil {
		log.Fatal(err)
	}
	conn, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()
	if err := auth.NewStore(conn).Ready(context.Background(), master); err != nil {
		log.Fatal(err)
	}

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

	// Same reasoning as the same-person-transfer backfill above: card-bill
	// payment legs synced before this rule existed need a one-time pass, and
	// it is logged rather than fatal for the same reason.
	if applied, err := categories.BackfillAutomaticCardPayment(context.Background(), conn); err != nil {
		log.Printf("backfill card payment categories failed, leaving those rows uncategorized: %v", err)
	} else if applied > 0 {
		log.Printf("categorized %d card payment transactions as transfers", applied)
	}

	frontend, err := webui.DistFS()
	if err != nil {
		log.Fatalf("load embedded frontend: %v", err)
	}

	secrets := settings.NewSecrets(master)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx, conn, secrets, worker.Config{
		Pluggy:                pluggy.DefaultConfig(),
		OnTransactionUpserted: automation.NewTransactionHook(payables.UnlinkIfPresent),
	})

	if scheduled {
		log.Printf("sync scheduled daily at %02d:%02d %s", schedule.Hour, schedule.Minute, schedule.Location)
		go worker.RunSchedule(ctx, conn, schedule)
	}

	handler := httpapi.NewServer(conn, frontend, secrets, config)
	log.Printf("listening on http://%s", *addr)
	server := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
