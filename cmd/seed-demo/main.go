// Seed-demo — заполняет пустую БД демо-данными лазерного сервиса одной командой (AC-019,
// make seed-demo). Тонкая обёртка: DATABASE_URL → seeddemo.Run (эталон cmd/seed/main.go).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/internal/seeddemo"
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

	res, err := seeddemo.Run(ctx, pool)
	if err != nil {
		if errors.Is(err, seeddemo.ErrNoAdmin) {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, "подсказка: make local-up-seed (создаёт seed-админа), затем make seed-demo")
		} else {
			fmt.Fprintln(os.Stderr, "seed-demo:", err)
		}
		os.Exit(1)
	}

	if res.Skipped {
		fmt.Println("seed-demo:", res.Message)
		return
	}
	fmt.Printf("seed-demo: %s (исполнители: %d, клиенты: %d, оборудование: %d, товары: %d, услуги: %d, заказы: %d)\n",
		res.Message, res.Executors, res.Clients, res.Equipment, res.Products, res.Services, res.Tasks)
}
