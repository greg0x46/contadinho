package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/greg0x46/julius/internal/db"
	"github.com/greg0x46/julius/internal/investments"
)

func seedCommand(args []string) error {
	if len(args) == 0 || args[0] != "investment-assets" {
		return fmt.Errorf("uso: julius seed investment-assets [-db caminho ou DSN]")
	}
	flags := flag.NewFlagSet("seed investment-assets", flag.ContinueOnError)
	defaultDB := db.DefaultPath()
	path := flags.String("db", defaultDB, "arquivo SQLite ou DSN Postgres")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("argumentos inesperados")
	}
	conn, err := db.Open(*path)
	if err != nil {
		return err
	}
	defer conn.Close()
	inserted, err := investments.SeedInvestmentAssets(context.Background(), conn)
	if err != nil {
		return err
	}
	fmt.Printf("Catálogo de ativos: %d novos cadastros.\n", inserted)
	return nil
}
