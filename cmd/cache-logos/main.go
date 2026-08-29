package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/database"
)

func downloadIfMissing(url, filename string) error {
	if info, err := os.Stat(filename); err == nil {
		log.Printf("Déjà en cache : %s (%d octets)", filename, info.Size())
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("vérification %s: %w", filename, err)
	}

	log.Printf("Téléchargement : %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("téléchargement %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"HTTP %d pour %s",
			resp.StatusCode,
			url,
		)
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("création répertoire %s: %w", filepath.Dir(filename), err)
	}

	tmp := filename + ".tmp"

	file, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("création %s: %w", tmp, err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		os.Remove(tmp)
		return fmt.Errorf("écriture %s: %w", tmp, err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("fermeture %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, filename); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("installation %s: %w", filename, err)
	}

	info, err := os.Stat(filename)
	if err == nil {
		log.Printf("  -> %s (%d octets)", filename, info.Size())
	}

	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erreur chargement configuration : %v", err)
	}

	dbFile := filepath.Join(cfg.DataDir, "epg.db")
	logoRoot := filepath.Join(cfg.DataDir, "logos")

	db, err := database.Open(dbFile)
	if err != nil {
		log.Fatalf("Erreur ouverture SQLite : %v", err)
	}
	defer db.Close()

	if err := database.Initialize(db); err != nil {
		log.Fatalf("Erreur initialisation SQLite : %v", err)
	}

	channels, err := database.GetAllChannels(db)
	if err != nil {
		log.Fatalf("Erreur lecture des chaînes : %v", err)
	}

	log.Printf("Chaînes trouvées dans SQLite : %d", len(channels))

	var (
		colorDownloaded int
		monoDownloaded  int
		colorCached     int
		monoCached      int
		errors          int
	)

	for _, channel := range channels {
		dir := filepath.Join(logoRoot, fmt.Sprintf("%d", channel.ID))

		// --------------------------------------------------------
		// Logo couleur — type 10
		// --------------------------------------------------------

		if channel.ColorLogoID > 0 {
			filename := filepath.Join(dir, "color.png")

			url := fmt.Sprintf(
				"%s?id=%d&t=10",
				cfg.ImageURL,
				channel.ColorLogoID,
			)

			if _, err := os.Stat(filename); err == nil {
				log.Printf(
					"[%d] %s : couleur déjà en cache",
					channel.AccessNum,
					channel.Name,
				)
				colorCached++
			} else {
				if err := downloadIfMissing(url, filename); err != nil {
					log.Printf(
						"ERREUR logo couleur [%d] %s : %v",
						channel.ID,
						channel.Name,
						err,
					)
					errors++
				} else {
					colorDownloaded++
				}
			}
		}

		// --------------------------------------------------------
		// Logo monochrome — type 8
		// --------------------------------------------------------

		if channel.MonoLogoID > 0 {
			filename := filepath.Join(dir, "mono.png")
			url := fmt.Sprintf(
				"%s?id=%d&t=8",
				cfg.ImageURL,
				channel.ColorLogoID,
			)

			if _, err := os.Stat(filename); err == nil {
				log.Printf(
					"[%d] %s : mono déjà en cache",
					channel.AccessNum,
					channel.Name,
				)
				monoCached++
			} else {
				if err := downloadIfMissing(url, filename); err != nil {
					log.Printf(
						"ERREUR logo mono [%d] %s : %v",
						channel.ID,
						channel.Name,
						err,
					)
					errors++
				} else {
					monoDownloaded++
				}
			}
		}
	}

	log.Println()
	log.Println("========================================")
	log.Println("CACHE DES LOGOS TERMINÉ")
	log.Println("========================================")
	log.Printf("Chaînes SQLite        : %d", len(channels))
	log.Printf("Logos couleur ajoutés : %d", colorDownloaded)
	log.Printf("Logos couleur cache   : %d", colorCached)
	log.Printf("Logos mono ajoutés    : %d", monoDownloaded)
	log.Printf("Logos mono cache      : %d", monoCached)
	log.Printf("Erreurs               : %d", errors)
	log.Println("========================================")
}
