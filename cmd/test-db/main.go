package main

import (
	"log"

	"epg-vmedia/internal/database"
)

func main() {
	db, err := database.Open("/data/epg.db")
	if err != nil {
		log.Fatalf("Erreur ouverture SQLite : %v", err)
	}
	defer db.Close()

	if err := database.Initialize(db); err != nil {
		log.Fatalf("Erreur initialisation SQLite : %v", err)
	}

	log.Println("Base SQLite initialisée avec succès.")
}
