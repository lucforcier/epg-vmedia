package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/database"
	"epg-vmedia/internal/logos"
	"epg-vmedia/internal/vmedia"
)

func logoID(channel vmedia.Channel, imageType int) int {
	for _, image := range channel.Images {
		if image.Type == imageType && image.ID != 0 {
			return image.ID
		}
	}

	return 0
}

func main() {
	// ------------------------------------------------------------
	// Options de ligne de commande
	// ------------------------------------------------------------

	refreshLogos := flag.Bool(
		"refresh-logos",
		false,
		"Forcer le téléchargement de tous les logos",
	)

	flag.Parse()

	// ------------------------------------------------------------
	// Charger la configuration
	// ------------------------------------------------------------

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erreur chargement configuration : %v", err)
	}

	databaseFile := filepath.Join(cfg.DataDir, "epg.db")
	logoDir := filepath.Join(cfg.DataDir, "logos")

	log.Println("========================================")
	log.Println("IMPORT DES CHAÎNES")
	log.Println("========================================")
	log.Printf("Fournisseur : %s", cfg.Provider)
	log.Printf("API         : %s", cfg.APIURL)
	log.Printf("Site ID     : %d", cfg.SiteID)
	log.Printf("AppName     : %s", cfg.AppName)
	log.Printf("Langue      : %s", cfg.Language)
	log.Printf("Répertoire de données : %s", cfg.DataDir)
	log.Printf("Base SQLite : %s", databaseFile)

	if *refreshLogos {
		log.Println("Mode logos : RAFRAÎCHISSEMENT FORCÉ")
	} else {
		log.Println("Mode logos : CACHE NORMAL")
	}

	log.Println()

	// ------------------------------------------------------------
	// Ouvrir / créer SQLite
	// ------------------------------------------------------------

	db, err := database.Open(databaseFile)
	if err != nil {
		log.Fatalf("Erreur ouverture SQLite : %v", err)
	}
	defer db.Close()

	if err := database.Initialize(db); err != nil {
		log.Fatalf("Erreur initialisation SQLite : %v", err)
	}

	// ------------------------------------------------------------
	// Déterminer si c'est le premier import
	// ------------------------------------------------------------

	var existingCount int

	if err := db.QueryRow(
		"SELECT COUNT(*) FROM channels",
	).Scan(&existingCount); err != nil {
		log.Fatalf("Erreur comptage chaînes existantes : %v", err)
	}

	firstImport := existingCount == 0

	if firstImport {
		log.Println("Base SQLite vide : premier import")
	} else {
		log.Printf("Chaînes déjà présentes dans SQLite : %d", existingCount)
	}

	// ------------------------------------------------------------
	// Récupérer les chaînes depuis l'API
	// ------------------------------------------------------------

	client := vmedia.NewAPIClient(cfg)

	channels, err := client.GetChannels()
	if err != nil {
		log.Fatalf("Erreur récupération des chaînes API : %v", err)
	}

	log.Printf("Chaînes retournées par l'API : %d", len(channels))
	log.Println()

	// ------------------------------------------------------------
	// Import transactionnel
	// ------------------------------------------------------------

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Erreur début transaction : %v", err)
	}

	defer tx.Rollback()

	const query = `
INSERT INTO channels (
	id,
	access_num,
	name,
	callsign,
	provider_channel_id,
	visible,
	enabled,
	logo_preference,
	color_logo_id,
	mono_logo_id,
	first_seen,
	last_seen,
	is_new,
	is_base
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	access_num = excluded.access_num,
	name = excluded.name,
	callsign = excluded.callsign,
	provider_channel_id = excluded.provider_channel_id,
	visible = excluded.visible,
	color_logo_id = excluded.color_logo_id,
	mono_logo_id = excluded.mono_logo_id,
	last_seen = excluded.last_seen
`

	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Fatalf("Erreur préparation SQL : %v", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)

	imported := 0
	colorLogos := 0
	monoLogos := 0
	visibleChannels := 0
	newChannels := 0

	// ------------------------------------------------------------
	// Importer chaque chaîne
	// ------------------------------------------------------------

	for _, channel := range channels {
		colorLogo := logoID(channel, 10)
		monoLogo := logoID(channel, 8)

		if colorLogo != 0 {
			colorLogos++
		}

		if monoLogo != 0 {
			monoLogos++
		}

		visible := 0
		if channel.Visible {
			visible = 1
			visibleChannels++
		}

		// --------------------------------------------------------
		// Déterminer si la chaîne est nouvelle
		// --------------------------------------------------------

		isNew := 0

		if !firstImport {
			var exists int

			err := tx.QueryRow(
				"SELECT COUNT(*) FROM channels WHERE id = ?",
				channel.ID,
			).Scan(&exists)

			if err != nil {
				log.Fatalf(
					"Erreur vérification chaîne %d (%s) : %v",
					channel.ID,
					channel.Name,
					err,
				)
			}

			if exists == 0 {
				isNew = 1
				newChannels++

				log.Printf(
					"NOUVELLE CHAÎNE : %d — %s",
					channel.AccessNum,
					channel.Name,
				)
			}
		}

		// --------------------------------------------------------
		// INSERT / UPDATE
		//
		// IMPORTANT :
		// Lors d'un UPDATE, on ne touche PAS à :
		//   enabled
		//   logo_preference
		//   is_new
		//   is_base
		//
		// Ces valeurs appartiennent à l'utilisateur.
		// --------------------------------------------------------

		_, err = stmt.Exec(
			channel.ID,
			channel.AccessNum,
			channel.Name,
			channel.CallSign,
			channel.ProviderChannelID,
			visible,
			0,
			"color",
			colorLogo,
			monoLogo,
			now,
			now,
			isNew,
			0,
		)

		if err != nil {
			log.Fatalf(
				"Erreur import chaîne %d (%s) : %v",
				channel.ID,
				channel.Name,
				err,
			)
		}

		imported++
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Erreur validation transaction : %v", err)
	}

	// ------------------------------------------------------------
	// Vérifications
	// ------------------------------------------------------------

	var count int

	if err := db.QueryRow(
		"SELECT COUNT(*) FROM channels",
	).Scan(&count); err != nil {
		log.Fatalf("Erreur comptage SQLite : %v", err)
	}

	var enabledCount int

	if err := db.QueryRow(
		"SELECT COUNT(*) FROM channels WHERE enabled = 1",
	).Scan(&enabledCount); err != nil {
		log.Fatalf("Erreur comptage enabled : %v", err)
	}

	var currentNewCount int

	if err := db.QueryRow(
		"SELECT COUNT(*) FROM channels WHERE is_new = 1",
	).Scan(&currentNewCount); err != nil {
		log.Fatalf("Erreur comptage nouvelles chaînes : %v", err)
	}

	var baseCount int

	if err := db.QueryRow(
		"SELECT COUNT(*) FROM channels WHERE is_base = 1",
	).Scan(&baseCount); err != nil {
		log.Fatalf("Erreur comptage chaînes de base : %v", err)
	}

	log.Println()
	log.Println("========================================")
	log.Println("IMPORT SQLITE TERMINÉ")
	log.Println("========================================")
	log.Printf("Premier import           : %v", firstImport)
	log.Printf("Chaînes API              : %d", len(channels))
	log.Printf("Chaînes traitées         : %d", imported)
	log.Printf("Chaînes dans SQLite      : %d", count)
	log.Printf("Chaînes visibles         : %d", visibleChannels)
	log.Printf("Chaînes enabled          : %d", enabledCount)
	log.Printf("Chaînes de base          : %d", baseCount)
	log.Printf("Nouvelles détectées      : %d", newChannels)
	log.Printf("Nouvelles actuellement   : %d", currentNewCount)
	log.Printf("Logos couleur (type 10) : %d", colorLogos)
	log.Printf("Logos mono (type 8)      : %d", monoLogos)
	log.Println("========================================")

	// ------------------------------------------------------------
	// Cache des logos
	// ------------------------------------------------------------

	log.Println()
	log.Println("========================================")
	log.Println("CACHE DES LOGOS")
	log.Println("========================================")

	if *refreshLogos {
		log.Println("MODE : RAFRAÎCHISSEMENT FORCÉ")
	} else {
		log.Println("MODE : CACHE NORMAL")
	}

	log.Printf("Répertoire : %s", logoDir)

	cache := logos.NewCache(logoDir, cfg.ImageURL)

	var (
		colorOK  int
		monoOK   int
		logoErrs int
	)

	for _, channel := range channels {

		// --------------------------------------------------------
		// Logo couleur — type 10
		// --------------------------------------------------------

		colorLogoID := logoID(channel, 10)

		if colorLogoID > 0 {

			var err error

			if *refreshLogos {
				err = cache.RefreshColor(
					channel.ID,
					colorLogoID,
				)
			} else {
				err = cache.DownloadColor(
					channel.ID,
					colorLogoID,
				)
			}

			if err != nil {
				log.Printf(
					"ERREUR logo couleur [%d] %s : %v",
					channel.ID,
					channel.Name,
					err,
				)
				logoErrs++
			} else {
				colorOK++
			}
		}

		// --------------------------------------------------------
		// Logo monochrome — type 8
		// --------------------------------------------------------

		monoLogoID := logoID(channel, 8)

		if monoLogoID > 0 {

			var err error

			if *refreshLogos {
				err = cache.RefreshMono(
					channel.ID,
					monoLogoID,
				)
			} else {
				err = cache.DownloadMono(
					channel.ID,
					monoLogoID,
				)
			}

			if err != nil {
				log.Printf(
					"ERREUR logo mono [%d] %s : %v",
					channel.ID,
					channel.Name,
					err,
				)
				logoErrs++
			} else {
				monoOK++
			}
		}
	}

	log.Println()
	log.Println("========================================")
	log.Println("IMPORT + CACHE TERMINÉS")
	log.Println("========================================")
	log.Printf("Chaînes SQLite       : %d", count)
	log.Printf("Logos couleur traités : %d", colorOK)
	log.Printf("Logos mono traités    : %d", monoOK)
	log.Printf("Erreurs logos         : %d", logoErrs)
	log.Println("========================================")
}
