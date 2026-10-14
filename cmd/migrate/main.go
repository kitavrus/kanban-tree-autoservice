// Migrate — go run ./cmd/migrate up|down (без внешнего migrate-CLI).
// DATABASE_URL читается напрямую (не internal/config): миграции не обязаны знать
// auth-режимные переменные (JWT_SECRET и т.п.), которые config.Load() требует.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		fmt.Fprintln(os.Stderr, "usage: migrate up|down")
		os.Exit(1)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL обязателен")
		os.Exit(1)
	}
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate init:", err)
		os.Exit(1)
	}
	if os.Args[1] == "up" {
		err = m.Up()
	} else {
		err = m.Steps(-1)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(os.Stderr, "migrate", os.Args[1]+":", err)
		os.Exit(1)
	}
	fmt.Println("migrate", os.Args[1], "OK")
}
