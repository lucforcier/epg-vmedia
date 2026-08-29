package main

import (
	"log"
	"path/filepath"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/logos"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erreur chargement configuration : %v", err)
	}

	logoDir := filepath.Join(cfg.DataDir, "logos")

	cache := logos.NewCache(
		logoDir,
		cfg.ImageURL,
	)

	const channelID = 303117
	const colorLogoID = 10875987
	const monoLogoID = 10875992

	log.Println("Téléchargement du logo couleur...")
	if err := cache.DownloadColor(channelID, colorLogoID); err != nil {
		log.Fatalf("Erreur logo couleur : %v", err)
	}

	log.Println("Téléchargement du logo monochrome...")
	if err := cache.DownloadMono(channelID, monoLogoID); err != nil {
		log.Fatalf("Erreur logo monochrome : %v", err)
	}

	log.Println("Cache de logos créé avec succès.")
}
