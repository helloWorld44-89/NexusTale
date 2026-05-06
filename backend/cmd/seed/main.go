// seed — one-time CLI to create the owner account on a fresh alpha/prod DB.
//
// Usage (SSH tunnel from local machine):
//
//	ssh -L 5432:localhost:5432 ubuntu@<ec2-ip> -N &
//	SEED_DB_URL="postgres://nexustale:<password>@localhost:5432/nexustale?sslmode=disable" \
//	SEED_EMAIL="you@example.com" \
//	SEED_PASSWORD="your-password" \
//	SEED_DISPLAY_NAME="Your Name" \
//	go run ./backend/cmd/seed
//
// Running it a second time with the same email is safe — the plan is updated
// but the password is left unchanged.
//
// After seeding, kill the SSH tunnel: kill %1
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbURL := requireEnv("SEED_DB_URL")
	email := requireEnv("SEED_EMAIL")
	password := requireEnv("SEED_PASSWORD")
	displayName := envOrDefault("SEED_DISPLAY_NAME", strings.Split(email, "@")[0])
	plan := envOrDefault("SEED_PLAN", "chronicler")

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("cannot connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("database ping failed", "error", err)
		os.Exit(1)
	}

	// Check if user already exists.
	var existingID string
	err = pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&existingID)
	if err == nil {
		// User exists — update plan only (don't touch the password).
		_, err = pool.Exec(ctx,
			`UPDATE users SET plan = $1, updated_at = NOW() WHERE email = $2`,
			plan, email,
		)
		if err != nil {
			slog.Error("failed to update plan", "error", err)
			os.Exit(1)
		}
		slog.Info("user already exists — plan updated", "email", email, "plan", plan, "id", existingID)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		os.Exit(1)
	}

	var id string
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, password_hash, role, plan)
		VALUES ($1, $2, $3, 'author', $4)
		RETURNING id
	`, email, displayName, string(hash), plan).Scan(&id)
	if err != nil {
		slog.Error("failed to insert user", "error", err)
		os.Exit(1)
	}

	slog.Info("owner account created",
		"email", email,
		"display_name", displayName,
		"plan", plan,
		"id", id,
	)
	fmt.Printf("\nDone. Log in at https://<your-domain> with:\n  Email:    %s\n  Password: (as supplied)\n\n", email)
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
