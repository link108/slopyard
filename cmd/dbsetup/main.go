package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	targetConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		log.Fatal(err)
	}

	targetDatabase := targetConfig.Database
	if targetDatabase == "" {
		log.Fatal("DATABASE_URL must include a database name")
	}

	setupURL := os.Getenv("SETUP_DATABASE_URL")
	setupConfig := targetConfig.Copy()
	if setupURL != "" {
		setupConfig, err = pgx.ParseConfig(setupURL)
		if err != nil {
			log.Fatal(err)
		}
	}
	setupConfig.Database = "postgres"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := pgx.ConnectConfig(ctx, setupConfig)
	if err != nil {
		log.Fatalf("connect to postgres maintenance database: %v", err)
	}
	defer conn.Close(ctx)

	if targetConfig.User != "" {
		if err := ensureRole(ctx, conn, targetConfig.User, targetConfig.Password); err != nil {
			log.Fatal(err)
		}
	}

	var exists bool
	if err := conn.QueryRow(ctx, "select exists(select 1 from pg_database where datname = $1)", targetDatabase).Scan(&exists); err != nil {
		log.Fatal(err)
	}

	if exists {
		fmt.Printf("database %q already exists\n", targetDatabase)
		return
	}

	createSQL := "create database " + pgx.Identifier{targetDatabase}.Sanitize()
	if targetConfig.User != "" {
		createSQL += " owner " + pgx.Identifier{targetConfig.User}.Sanitize()
	}
	if _, err := conn.Exec(ctx, createSQL); err != nil {
		log.Fatalf("create database %q: %v", targetDatabase, err)
	}

	fmt.Printf("created database %q\n", targetDatabase)
}

func ensureRole(ctx context.Context, conn *pgx.Conn, role string, password string) error {
	var exists bool
	if err := conn.QueryRow(ctx, "select exists(select 1 from pg_roles where rolname = $1)", role).Scan(&exists); err != nil {
		return err
	}
	if exists {
		fmt.Printf("role %q already exists\n", role)
		return nil
	}

	createSQL := "create role " + pgx.Identifier{role}.Sanitize() + " login"
	if password != "" {
		createSQL += " password " + quoteLiteral(password)
	}
	if _, err := conn.Exec(ctx, createSQL); err != nil {
		return fmt.Errorf("create role %q: %w", role, err)
	}

	fmt.Printf("created role %q\n", role)
	return nil
}

func quoteLiteral(value string) string {
	quoted := "'"
	for _, r := range value {
		if r == '\'' {
			quoted += "''"
			continue
		}
		quoted += string(r)
	}
	return quoted + "'"
}
