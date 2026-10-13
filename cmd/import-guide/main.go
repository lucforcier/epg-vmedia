package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/database"
	"epg-vmedia/internal/vmedia"
)

const (
	guideMaxRetries     = 5
	guideRetryBaseDelay = 2 * time.Second
)

func main() {

	channelID := flag.Int(
		"channel",
		0,
		"ID VMedia d'une chaîne à importer (0 = toutes les chaînes activées)",
	)

	hours := flag.Int(
		"hours",
		24,
		"nombre d'heures à récupérer",
	)

	pageSize := flag.Int(
		"page-size",
		500,
		"nombre d'éléments par page API",
	)

	flag.Parse()

	// ------------------------------------------------------------
	// Configuration
	// ------------------------------------------------------------

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// ------------------------------------------------------------
	// SQLite
	// ------------------------------------------------------------

	dbPath := cfg.DataDir + "/epg.db"

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.InitializeEPGSchema(db); err != nil {
		log.Fatal(err)
	}

	// ------------------------------------------------------------
	// Chaînes activées
	// ------------------------------------------------------------

	channels, err := database.GetAllChannels(db)
	if err != nil {
		log.Fatal(err)
	}

	enabled := make(map[int]bool)

	for _, ch := range channels {
		if ch.Enabled {
			enabled[ch.ID] = true
		}
	}

	// Si une chaîne précise est demandée avec -channel,
	// elle doit être activée dans SQLite.
	if *channelID != 0 {

		if !enabled[*channelID] {
			log.Fatalf(
				"la chaîne %d n'est pas activée dans SQLite",
				*channelID,
			)
		}

		enabled = map[int]bool{
			*channelID: true,
		}
	}

	// ------------------------------------------------------------
	// Filtre de chaînes transmis directement à l'API VMedia
	// ------------------------------------------------------------

	channelFilter := make([]int, 0, len(enabled))

	for id := range enabled {
		channelFilter = append(channelFilter, id)
	}

	log.Printf(
		"Chaînes à importer : %d",
		len(channelFilter),
	)
	if len(channelFilter) == 0 {
		log.Fatal("aucune chaîne activée dans SQLite ; import du guide impossible")
	}

	// ------------------------------------------------------------
	// API VMedia
	// ------------------------------------------------------------

	client := vmedia.NewAPIClient(cfg)

	start := time.Now().UTC()
	end := start.Add(time.Duration(*hours) * time.Hour)

	log.Printf(
		"Guide : %s → %s",
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)

	// ------------------------------------------------------------
	// Pagination
	//
	// IMPORTANT :
	// Une transaction SQLite est ouverte pour CHAQUE PAGE API,
	// et non pour chaque événement.
	//
	// Ainsi :
	//
	//   GET page 1
	//       BEGIN
	//       import des événements
	//       COMMIT
	//
	//   GET page 2
	//       BEGIN
	//       import des événements
	//       COMMIT
	//
	// Une panne réseau lors de la page N laisse toutes les pages
	// précédentes définitivement enregistrées dans SQLite.
	// ------------------------------------------------------------

	totalImported := 0
	totalIgnored := 0
	totalSkipped := 0

	for page := 1; ; page++ {

		// --------------------------------------------------------
		// Récupération réseau
		//
		// Aucun verrou SQLite / aucune transaction pendant
		// l'appel à l'API.
		// --------------------------------------------------------

		var (
			items      []vmedia.GuideItem
			total      int
			totalPages int
			err        error
		)

		for attempt := 1; attempt <= guideMaxRetries; attempt++ {

			items, total, totalPages, err = client.GetGuide(
				start,
				end,
				page,
				*pageSize,
				channelFilter,
			)

			if err == nil {
				break
			}

			if attempt == guideMaxRetries {
				log.Printf(
					"Page %d : échec après %d tentatives : %v",
					page,
					guideMaxRetries,
					err,
				)

				log.Fatalf(
					"Import interrompu après échec de la page %d : les pages précédentes ont été conservées dans SQLite : %v",
					page,
					err,
				)
			}

			delay := guideRetryBaseDelay * time.Duration(1<<(attempt-1))

			log.Printf(
				"Page %d : tentative %d/%d échouée : %v",
				page,
				attempt,
				guideMaxRetries,
				err,
			)

			log.Printf(
				"Page %d : nouvelle tentative dans %s",
				page,
				delay,
			)

			time.Sleep(delay)
		}
		log.Printf(
			"Page %d/%d : %d éléments (API total=%d)",
			page,
			totalPages,
			len(items),
			total,
		)

		if len(items) == 0 {
			break
		}

		// --------------------------------------------------------
		// Une transaction par page
		// --------------------------------------------------------

		tx, err := db.Begin()
		if err != nil {
			log.Fatalf(
				"impossible de commencer la transaction page %d : %v",
				page,
				err,
			)
		}

		pageImported := 0
		pageIgnored := 0
		pageSkipped := 0

		pageOK := true

		// --------------------------------------------------------
		// Import des événements de la page
		// --------------------------------------------------------

		for _, item := range items {

			// ----------------------------------------------------
			// Garde-fou : l'API devrait déjà avoir filtré.
			// ----------------------------------------------------

			if !enabled[item.RecentAirTime.ChannelID] {
				pageIgnored++
				continue
			}

			// ----------------------------------------------------
			// ProgramID obligatoire
			// ----------------------------------------------------

			if item.ProgramID == 0 {
				log.Printf(
					"Programme sans ProgramID ignoré : event=%d name=%q",
					item.ID,
					item.Name,
				)

				pageSkipped++
				continue
			}

			// ----------------------------------------------------
			// Conversion du timestamp VMedia
			// ----------------------------------------------------

			startTime, err := vmedia.ParseVMediaDate(
				item.StartTime,
			)

			if err != nil {
				log.Printf(
					"Timestamp invalide event=%d : %v",
					item.ID,
					err,
				)

				pageSkipped++
				continue
			}

			// ----------------------------------------------------
			// JSON source
			// ----------------------------------------------------

			sourceJSON, err := database.MarshalGuideItem(item)
			if err != nil {
				log.Printf(
					"JSON impossible event=%d : %v",
					item.ID,
					err,
				)

				pageSkipped++
				continue
			}

			// ----------------------------------------------------
			// Container / série
			// ----------------------------------------------------

			containerID := sql.NullInt64{}

			if item.ContainerID != 0 {

				id, err := database.UpsertContainer(
					tx,
					item,
					sourceJSON,
				)

				if err != nil {
					log.Printf(
						"container event=%d : %v",
						item.ID,
						err,
					)

					pageOK = false
					break
				}

				containerID = sql.NullInt64{
					Int64: id,
					Valid: true,
				}
			}

			// ----------------------------------------------------
			// Programme
			// ----------------------------------------------------

			programID, err := database.UpsertProgram(
				tx,
				item,
				containerID,
				sourceJSON,
			)

			if err != nil {
				log.Printf(
					"programme event=%d : %v",
					item.ID,
					err,
				)

				pageOK = false
				break
			}

			// ----------------------------------------------------
			// Images du programme
			// ----------------------------------------------------

			for _, image := range item.Images {

				imageID, err := database.UpsertImage(
					tx,
					image,
				)

				if err != nil {
					log.Printf(
						"image programme event=%d : %v",
						item.ID,
						err,
					)

					pageOK = false
					break
				}

				if err := database.LinkProgramImage(
					tx,
					programID,
					imageID,
				); err != nil {
					log.Printf(
						"relation programme/image event=%d : %v",
						item.ID,
						err,
					)

					pageOK = false
					break
				}
			}

			if !pageOK {
				break
			}

			// ----------------------------------------------------
			// Images de série
			// ----------------------------------------------------

			if containerID.Valid {

				for _, image := range item.SeriesImages {

					imageID, err := database.UpsertImage(
						tx,
						image,
					)

					if err != nil {
						log.Printf(
							"image série event=%d : %v",
							item.ID,
							err,
						)

						pageOK = false
						break
					}

					if err := database.LinkSeriesImage(
						tx,
						containerID.Int64,
						imageID,
					); err != nil {
						log.Printf(
							"relation série/image event=%d : %v",
							item.ID,
							err,
						)

						pageOK = false
						break
					}
				}
			}

			if !pageOK {
				break
			}

			// ----------------------------------------------------
			// Événement
			// ----------------------------------------------------

			if err := database.InsertEvent(
				tx,
				item,
				item.RecentAirTime.ChannelID,
				programID,
				containerID,
				startTime.Format(time.RFC3339),
				sourceJSON,
			); err != nil {
				log.Printf(
					"event event=%d : %v",
					item.ID,
					err,
				)

				pageOK = false
				break
			}

			pageImported++
		}

		// --------------------------------------------------------
		// Si une erreur SQLite est survenue :
		//
		// Toute la page est annulée.
		//
		// Les pages précédentes restent commitées.
		// --------------------------------------------------------

		if !pageOK {

			if err := tx.Rollback(); err != nil {
				log.Printf(
					"erreur rollback page %d : %v",
					page,
					err,
				)
			}

			log.Fatalf(
				"page %d annulée : aucun de ses %d événements importés n'a été commité",
				page,
				pageImported,
			)
		}

		// --------------------------------------------------------
		// Commit de la page complète
		// --------------------------------------------------------

		if err := tx.Commit(); err != nil {
			log.Fatalf(
				"erreur commit page %d : %v",
				page,
				err,
			)
		}

		totalImported += pageImported
		totalIgnored += pageIgnored
		totalSkipped += pageSkipped

		log.Printf(
			"Page %d commitée : %d importés, %d ignorés, %d sautés",
			page,
			pageImported,
			pageIgnored,
			pageSkipped,
		)

		// --------------------------------------------------------
		// Fin de la pagination
		// --------------------------------------------------------

		if page >= totalPages {
			break
		}
	}

	if totalImported == 0 {
		log.Fatal("aucun événement importé ; génération XMLTV annulée pour éviter de publier un guide vide")
	}

// ------------------------------------------------------------
	// Résultat
	// ------------------------------------------------------------

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("IMPORT TERMINÉ")
	fmt.Println("========================================")
	fmt.Printf("Événements importés : %d\n", totalImported)
	fmt.Printf("Éléments ignorés    : %d\n", totalIgnored)
	fmt.Printf("Éléments sautés     : %d\n", totalSkipped)
	fmt.Println("========================================")
}
