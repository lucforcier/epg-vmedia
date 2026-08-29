package main

import (
	"fmt"
	"log"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	databaseFile := cfg.DataDir + "/epg.db"

	fmt.Println("========================================")
	fmt.Println("INITIALISATION SQLITE EPG")
	fmt.Println("========================================")
	fmt.Println("Base :", databaseFile)

	db, err := database.Open(databaseFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.InitializeEPGSchema(db); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Schéma EPG installé avec succès.")
}
