package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer m.Close()

	switch command {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "drop":
		err = m.Drop()
	case "version":
		version, dirty, versionErr := m.Version()
		if errors.Is(versionErr, migrate.ErrNilVersion) {
			fmt.Println("version: none")
			return
		}
		if versionErr != nil {
			log.Fatal(versionErr)
		}
		fmt.Printf("version: %d dirty: %t\n", version, dirty)
		return
	case "steps":
		if len(os.Args) < 3 {
			log.Fatal("usage: go run ./cmd/migrate steps <n>")
		}
		steps, parseErr := strconv.Atoi(os.Args[2])
		if parseErr != nil {
			log.Fatal(parseErr)
		}
		err = m.Steps(steps)
	default:
		log.Fatalf("unknown command %q; use up, down, drop, version, or steps", command)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no migration changes")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
}
