package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/EduThemes/paper-lms/internal/config"
	"github.com/EduThemes/paper-lms/internal/db"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	cmd := os.Args[1]

	switch cmd {
	case "up":
		if err := db.MigrateUp(database); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}

	case "down":
		steps := 1
		if len(os.Args) > 2 {
			steps, err = strconv.Atoi(os.Args[2])
			if err != nil {
				log.Fatalf("Invalid step count: %s", os.Args[2])
			}
		}
		if err := db.MigrateDown(database, steps); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
		fmt.Printf("Rolled back %d migration(s)\n", steps)

	case "version":
		version, dirty, err := db.MigrateVersion(database)
		if err != nil {
			fmt.Println("No migrations applied yet")
		} else {
			fmt.Printf("Version: %d (dirty: %v)\n", version, dirty)
		}

	case "force":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate force <version>")
		}
		version, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Invalid version: %s", os.Args[2])
		}
		if err := db.MigrateForce(database, version); err != nil {
			log.Fatalf("Force version failed: %v", err)
		}
		fmt.Printf("Forced migration version to %d\n", version)

	case "baseline":
		// For existing databases that were created with AutoMigrate:
		// marks them as having applied migration 1 (initial schema)
		if err := db.MigrateForce(database, 1); err != nil {
			log.Fatalf("Baseline failed: %v", err)
		}
		fmt.Println("Database baselined at migration version 1 (initial schema)")

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Paper LMS Database Migration Tool

Usage: migrate <command> [args]

Commands:
  up              Apply all pending migrations
  down [N]        Roll back N migrations (default: 1)
  version         Show current migration version
  force <V>       Force migration version to V (no migrations run)
  baseline        Mark existing database as having the initial schema (v1)

Environment:
  DATABASE_URL    PostgreSQL connection string`)
}
