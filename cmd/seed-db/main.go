package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/jeffersonwarrior/modelscan/scraper"
	"github.com/jeffersonwarrior/modelscan/storage"
)

func main() {
	dbPath := flag.String("db", "./rate_limits.db", "Path to rate limits database")
	flag.Parse()

	absPath, err := filepath.Abs(*dbPath)
	if err != nil {
		log.Fatalf("Invalid database path: %v", err)
	}

	log.Printf("Initializing database at: %s", absPath)
	if err := storage.InitRateLimitDB(absPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer storage.CloseRateLimitDB()

	log.Println("🚀 Starting seed process for 15 core providers...")

	// Seed rate limits
	if err := scraper.SeedInitialRateLimits(); err != nil {
		log.Fatalf("Failed to seed rate limits: %v", err)
	}

	// Seed pricing
	if err := scraper.SeedInitialPricing(); err != nil {
		log.Fatalf("Failed to seed pricing: %v", err)
	}

	log.Println("✅ Database seeded successfully!")
	log.Println("\nNext steps:")
	log.Println("  1. Review rate_limits.db with: sqlite3 rate_limits.db")
	log.Println("  2. Query rate limits: SELECT * FROM rate_limits WHERE provider_name='openai';")
	log.Println("  3. Query pricing: SELECT * FROM provider_pricing WHERE provider_name='anthropic';")
}
