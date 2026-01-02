package main

import (
	"entropy/cmd"
	"entropy/internal/db"
	"log"
)

func main() {
	if err := db.Init(); err != nil {
		log.Fatalf("CRITICAL: Failed to initialize local database: %v", err)
	}

	cmd.Execute()
}
