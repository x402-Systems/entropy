package main

import (
	"github.com/x402-Systems/entropy/cmd"
	"github.com/x402-Systems/entropy/internal/db"
	"log"
)

func main() {
	if err := db.Init(); err != nil {
		log.Fatalf("CRITICAL: Failed to initialize local database: %v", err)
	}

	cmd.Execute()
}
