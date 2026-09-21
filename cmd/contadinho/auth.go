package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"contadinho-go/internal/auth"
	"contadinho-go/internal/db"
	"golang.org/x/term"
)

func authCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: contadinho auth init|migrate|reset-password [-db caminho ou DSN]")
	}
	command := args[0]
	if command != "init" && command != "migrate" && command != "reset-password" {
		return fmt.Errorf("comando auth desconhecido")
	}
	flags := flag.NewFlagSet("auth "+command, flag.ContinueOnError)
	defaultDB := os.Getenv("CONTADINHO_DB")
	if defaultDB == "" {
		defaultDB = "contadinho.db"
	}
	path := flags.String("db", defaultDB, "arquivo SQLite ou DSN Postgres")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("argumentos inesperados")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("execute em um terminal interativo para entrada segura de senha")
	}
	var master []byte
	var err error
	if command != "reset-password" {
		master, err = auth.LoadMasterKey()
		if err != nil {
			return err
		}
	}
	var legacy *string
	if command == "migrate" {
		value, err := readPassword("Senha antiga de desbloqueio: ")
		if err != nil {
			return err
		}
		legacy = &value
	}
	email := ""
	if command != "reset-password" {
		fmt.Fprint(os.Stderr, "E-mail: ")
		email, err = bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		email = strings.TrimSpace(email)
	}
	password, err := readPassword("Nova senha: ")
	if err != nil {
		return err
	}
	confirmation, err := readPassword("Repita a nova senha: ")
	if err != nil {
		return err
	}
	if password != confirmation {
		return fmt.Errorf("as senhas não coincidem")
	}
	conn, err := db.Open(*path)
	if err != nil {
		return err
	}
	defer conn.Close()
	store := auth.NewStore(conn)
	if command == "reset-password" {
		err = store.ResetPassword(context.Background(), password)
	} else {
		err = store.Initialize(context.Background(), email, password, master, legacy)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Autenticação configurada. Faça login pelo navegador.")
	return nil
}
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(value), err
}
