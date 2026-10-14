// Seed — идемпотентный локальный администратор, role=admin (make local-up-seed). ON CONFLICT DO NOTHING:
// повторный прогон безопасен. Одна и та же users-таблица под session и jwt auth.mode.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	seedLogin    = "admin"
	seedPassword = "admin1234"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL обязателен")
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgx:", err)
		os.Exit(1)
	}
	defer pool.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bcrypt:", err)
		os.Exit(1)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO users (login, password_hash, name, role) VALUES ($1, $2, $3, 'admin')
		 ON CONFLICT (login) DO NOTHING`,
		seedLogin, string(hash), "Администратор")
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Printf("seed: %s / %s (готов, если не существовал)\n", seedLogin, seedPassword)
}
