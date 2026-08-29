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

	if err := database.SetChannelEnabled(db, 303117, true); err != nil {
		log.Fatalf("Erreur enabled : %v", err)
	}

	if err := database.SetLogoPreference(db, 303117, "mono"); err != nil {
		log.Fatalf("Erreur préférence logo : %v", err)
	}

	channels, err := database.GetAllChannels(db)
	if err != nil {
		log.Fatalf("Erreur lecture chaînes : %v", err)
	}

	for _, channel := range channels {
		if channel.ID == 303117 {
			log.Printf(
				"Chaîne %d : enabled=%v, logo_preference=%s",
				channel.ID,
				channel.Enabled,
				channel.LogoPreference,
			)
			return
		}
	}

	log.Fatal("Chaîne 303117 introuvable")
}
