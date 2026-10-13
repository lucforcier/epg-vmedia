package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/database"
	"epg-vmedia/internal/generator"
	"epg-vmedia/internal/images"
	"epg-vmedia/internal/scheduler"
)

func parseEPGChannels(value string) ([]int, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	channels := make([]int, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			return nil, fmt.Errorf(
				"ID de chaîne vide dans le paramètre channels",
			)
		}

		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf(
				"ID de chaîne invalide %q",
				part,
			)
		}

		channels = append(channels, id)
	}

	return channels, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erreur de configuration : %v", err)
	}

	log.Printf("Fournisseur EPG : %s", cfg.Provider)
	log.Printf("API VMedia : %s", cfg.APIURL)
	log.Printf("Site ID : %d", cfg.SiteID)
	log.Printf("AppName : %s", cfg.AppName)
	log.Printf("Langue : %s", cfg.Language)
	log.Printf("Répertoire de données : %s", cfg.DataDir)

	// ------------------------------------------------------------
	// SQLite
	// ------------------------------------------------------------

	databaseFile := filepath.Join(cfg.DataDir, "epg.db")

	db, err := database.Open(databaseFile)
	if err != nil {
		log.Fatalf("Erreur ouverture SQLite : %v", err)
	}
	defer db.Close()

	if err := database.Initialize(db); err != nil {
		log.Fatalf("Erreur initialisation SQLite : %v", err)
	}

	if err := database.InitializeEPGSchema(db); err != nil {
		log.Fatalf("Erreur initialisation du schéma EPG SQLite : %v", err)
	}

	// ------------------------------------------------------------
	// Génération XMLTV
	// ------------------------------------------------------------

	generateEPG := func() error {
		result, err := generator.Generate(
			db,
			generator.Options{
				Days:    14,
				Logo:    "",
				BaseURL: cfg.EPGBaseURL,
				Output:  cfg.EPGXMLPath,
			},
		)

		if err != nil {
			return err
		}

		log.Printf(
			"XMLTV généré : %s (%d chaînes, %d événements)",
			result.OutputPath,
			result.ChannelCount,
			result.EventCount,
		)

		return nil
	}

	// ------------------------------------------------------------
	// Importation du guide VMedia
	// ------------------------------------------------------------

	var guideRefreshMu sync.Mutex

	refreshGuide := func() error {
		guideRefreshMu.Lock()
		defer guideRefreshMu.Unlock()

		log.Println("Importation du guide VMedia : fenêtre glissante de 14 jours")

		cmd := exec.Command(
			"/opt/epg-vmedia/import-guide",
			"-hours",
			"336",
		)

		output, err := cmd.CombinedOutput()
		if len(output) > 0 {
			log.Printf("Sortie import-guide :\n%s", string(output))
		}
		if err != nil {
			return fmt.Errorf("import-guide (14 jours) : %w", err)
		}

		if err := generateEPG(); err != nil {
			return fmt.Errorf("génération XMLTV après import : %w", err)
		}
		return nil
	}

	// ------------------------------------------------------------
	// Cache des images de programmes
	// ------------------------------------------------------------

	imageCache := images.NewCache(
		filepath.Join(cfg.DataDir, "images"),
		cfg.ImageURL,
	)

	// ------------------------------------------------------------
	// Génération XMLTV au démarrage
	// ------------------------------------------------------------

	log.Println("========================================")
	log.Println("GÉNÉRATION XMLTV AU DÉMARRAGE")
	log.Println("========================================")

	if err := refreshGuide(); err != nil {
		// Le serveur web doit rester accessible au premier démarrage,
		// notamment pour permettre l'importation et la sélection des
		// chaînes depuis /channels. Le guide sera généré après activation
		// d'au moins une chaîne ou lors du prochain passage du scheduler.
		log.Printf(
			"Guide XMLTV non disponible au démarrage : %v",
			err,
		)
		log.Println(
			"Le serveur reste actif pour permettre la configuration des chaînes",
		)
	} else {
		log.Println(
			"Génération XMLTV au démarrage terminée avec succès",
		)
	}

	// ------------------------------------------------------------
	// Scheduler XMLTV
	// ------------------------------------------------------------

	// ------------------------------------------------------------
	// Rafraîchissement du catalogue des chaînes et logos
	// ------------------------------------------------------------

	var refreshChannelsMu sync.Mutex

	refreshChannels := func() error {
		// Le scheduler et l'endpoint HTTP partagent le même importateur.
		// Sérialiser les appels évite deux écritures concurrentes dans SQLite.
		refreshChannelsMu.Lock()
		defer refreshChannelsMu.Unlock()

		log.Println(
			"Rafraîchissement du catalogue des chaînes et des logos",
		)

		cmd := exec.Command(
			"/opt/epg-vmedia/import-channels",
			"--refresh-logos",
		)

		output, err := cmd.CombinedOutput()

		if len(output) > 0 {
			log.Printf(
				"Sortie import-channels :\n%s",
				string(output),
			)
		}

		if err != nil {
			return fmt.Errorf(
				"import-channels --refresh-logos : %w",
				err,
			)
		}

		return nil
	}

	// ------------------------------------------------------------
	// Nettoyage du cache des images
	// ------------------------------------------------------------

	cleanupImages := func() error {

		maxAge := time.Duration(
			cfg.ImageCacheMaxAgeDays,
		) * 24 * time.Hour

		removed, err := imageCache.Cleanup(maxAge)
		if err != nil {
			return fmt.Errorf(
				"nettoyage du cache des images : %w",
				err,
			)
		}

		log.Printf(
			"Nettoyage du cache des images terminé : %d fichiers supprimés (âge maximal : %d jours)",
			removed,
			cfg.ImageCacheMaxAgeDays,
		)

		return nil
	}

	epgScheduler, err := scheduler.New(
		cfg.SchedulerEnabled,
		cfg.SchedulerTime,
		cfg.SchedulerTimezone,
		refreshGuide,
	)
	if err != nil {
		log.Fatalf(
			"Erreur initialisation scheduler EPG : %v",
			err,
		)
	}

	epgScheduler.Start(context.Background())

	// ------------------------------------------------------------
	// Scheduler chaînes/logos hebdomadaire
	// ------------------------------------------------------------

	channelRefreshScheduler, err := scheduler.NewWeekly(
		cfg.ChannelRefreshEnabled,
		"chaînes et logos",
		cfg.ChannelRefreshDay,
		cfg.ChannelRefreshTime,
		cfg.SchedulerTimezone,
		refreshChannels,
	)
	if err != nil {
		log.Fatalf(
			"Erreur initialisation scheduler chaînes/logos : %v",
			err,
		)
	}

	channelRefreshScheduler.Start(context.Background())

	// ------------------------------------------------------------
	// Scheduler nettoyage cache images
	// ------------------------------------------------------------

	imageCleanupScheduler, err := scheduler.NewWeekly(
		cfg.ImageCleanupEnabled,
		"nettoyage cache images",
		cfg.ImageCleanupDay,
		cfg.ImageCleanupTime,
		cfg.SchedulerTimezone,
		cleanupImages,
	)
	if err != nil {
		log.Fatalf(
			"Erreur initialisation scheduler nettoyage images : %v",
			err,
		)
	}

	imageCleanupScheduler.Start(context.Background())

	// ------------------------------------------------------------
	// Template
	// ------------------------------------------------------------

	templateFile := filepath.Join(
		"/opt/epg-vmedia",
		"templates",
		"channels.html",
	)

	tmpl := template.Must(template.ParseFiles(templateFile))

	// ------------------------------------------------------------
	// Images de programmes — cache à la demande
	// ------------------------------------------------------------

	http.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(
				w,
				"Méthode non autorisée",
				http.StatusMethodNotAllowed,
			)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/images/")

		parts := strings.Split(
			strings.TrimSuffix(name, ".jpg"),
			"-",
		)

		if len(parts) != 2 {
			http.Error(
				w,
				"Nom d'image invalide",
				http.StatusBadRequest,
			)
			return
		}

		imageID, err := strconv.Atoi(parts[0])
		if err != nil || imageID <= 0 {
			http.Error(
				w,
				"ID d'image invalide",
				http.StatusBadRequest,
			)
			return
		}

		imageType, err := strconv.Atoi(parts[1])
		if err != nil || imageType <= 0 {
			http.Error(
				w,
				"Type d'image invalide",
				http.StatusBadRequest,
			)
			return
		}

		filename, err := imageCache.Get(imageID, imageType)
		if err != nil {
			log.Printf(
				"Erreur image %d type %d : %v",
				imageID,
				imageType,
				err,
			)

			http.Error(
				w,
				"Image indisponible",
				http.StatusBadGateway,
			)
			return
		}

		if r.Method == http.MethodHead {
			info, err := os.Stat(filename)
			if err != nil {
				http.Error(
					w,
					"Image indisponible",
					http.StatusInternalServerError,
				)
				return
			}

			w.Header().Set(
				"Content-Length",
				strconv.FormatInt(info.Size(), 10),
			)
			w.Header().Set("Content-Type", "image/jpeg")
			return
		}

		http.ServeFile(w, r, filename)
	})

	// ------------------------------------------------------------
	// Logos locaux
	// ------------------------------------------------------------

	logoDir := filepath.Join(cfg.DataDir, "logos")

	http.Handle(
		"/logos/",
		http.StripPrefix(
			"/logos/",
			http.FileServer(http.Dir(logoDir)),
		),
	)

	// ------------------------------------------------------------
	// Rafraîchissement du catalogue des chaînes
	// ------------------------------------------------------------

	http.HandleFunc("/channels/refresh", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(
				w,
				"Méthode non autorisée",
				http.StatusMethodNotAllowed,
			)
			return
		}

		log.Println("========================================")
		log.Println("RAFRAÎCHISSEMENT DES CHAÎNES DEMANDÉ PAR LE WEB")
		log.Println("========================================")

		if err := refreshChannels(); err != nil {
			log.Printf("Erreur rafraîchissement des chaînes : %v", err)
			http.Error(
				w,
				"Erreur lors du rafraîchissement des chaînes",
				http.StatusInternalServerError,
			)
			return
		}

		log.Println("Rafraîchissement des chaînes terminé")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// ------------------------------------------------------------
	// Page des chaînes
	// ------------------------------------------------------------

	http.HandleFunc("/channels", func(w http.ResponseWriter, r *http.Request) {

		// ============================================================
		// SAUVEGARDE
		// ============================================================

		if r.Method == http.MethodPost {

			if err := r.ParseForm(); err != nil {
				http.Error(
					w,
					"Formulaire invalide",
					http.StatusBadRequest,
				)
				return
			}

			// --------------------------------------------------------
			// Chaînes sélectionnées pour l'EPG
			// --------------------------------------------------------

			enabled := make(map[int]bool)

			for _, value := range r.Form["channel"] {
				var id int

				if _, err := fmt.Sscanf(value, "%d", &id); err != nil {
					log.Printf(
						"ID de chaîne invalide : %q",
						value,
					)
					continue
				}

				enabled[id] = true
			}

			// --------------------------------------------------------
			// Lecture de toutes les chaînes
			// --------------------------------------------------------

			channels, err := database.GetAllChannels(db)
			if err != nil {
				http.Error(
					w,
					"Impossible de lire les chaînes",
					http.StatusInternalServerError,
				)
				log.Printf(
					"Erreur lecture chaînes : %v",
					err,
				)
				return
			}

			processedCount := 0

			for _, channel := range channels {

				// ----------------------------------------------------
				// Sélection EPG
				// ----------------------------------------------------

				if err := database.SetChannelEnabled(
					db,
					channel.ID,
					enabled[channel.ID],
				); err != nil {
					http.Error(
						w,
						"Impossible de sauvegarder la sélection",
						http.StatusInternalServerError,
					)
					log.Printf(
						"Erreur sauvegarde chaîne %d : %v",
						channel.ID,
						err,
					)
					return
				}

				// ----------------------------------------------------
				// Préférence de logo
				// ----------------------------------------------------

				preference := r.FormValue(
					fmt.Sprintf(
						"logo_preference_%d",
						channel.ID,
					),
				)

				if preference == "" {
					preference = channel.LogoPreference
				}

				if err := database.SetLogoPreference(
					db,
					channel.ID,
					preference,
				); err != nil {
					http.Error(
						w,
						"Impossible de sauvegarder les préférences de logos",
						http.StatusInternalServerError,
					)
					log.Printf(
						"Erreur préférence logo chaîne %d : %v",
						channel.ID,
						err,
					)
					return
				}

				// ----------------------------------------------------
				// Chaîne de base / à la carte
				// ----------------------------------------------------

				baseValue := r.FormValue(
					fmt.Sprintf(
						"is_base_%d",
						channel.ID,
					),
				)

				isBase := baseValue == "1"

				if err := database.SetChannelBase(
					db,
					channel.ID,
					isBase,
				); err != nil {
					http.Error(
						w,
						"Impossible de sauvegarder le type de chaîne",
						http.StatusInternalServerError,
					)
					log.Printf(
						"Erreur type chaîne %d : %v",
						channel.ID,
						err,
					)
					return
				}

				// ----------------------------------------------------
				// Une chaîne nouvelle devient traitée dès qu'une
				// configuration explicite a été fournie.
				//
				// Le JavaScript envoie "processed_<id>" lorsqu'on
				// modifie logo ou statut base/à-la-carte.
				// ----------------------------------------------------

				processedValue := r.FormValue(
					fmt.Sprintf(
						"processed_%d",
						channel.ID,
					),
				)

				if processedValue == "1" && channel.IsNew {
					if err := database.MarkChannelProcessed(
						db,
						channel.ID,
					); err != nil {
						http.Error(
							w,
							"Impossible de marquer la chaîne comme traitée",
							http.StatusInternalServerError,
						)
						log.Printf(
							"Erreur marquage chaîne %d : %v",
							channel.ID,
							err,
						)
						return
					}

					processedCount++
				}
			}

			log.Printf(
				"Configuration SQLite sauvegardée : %d chaînes sélectionnées, %d nouvelles traitées",
				len(enabled),
				processedCount,
			)

			// Publier immédiatement le guide correspondant à la sélection
			// sauvegardée, plutôt que d'attendre la prochaine exécution planifiée.
			if err := generateEPG(); err != nil {
				log.Printf("Impossible de régénérer le guide après la sauvegarde des chaînes : %v", err)
			}

			http.Redirect(
				w,
				r,
				"/channels",
				http.StatusSeeOther,
			)
			return
		}

		// ============================================================
		// LECTURE SQLITE
		// ============================================================

		channels, err := database.GetAllChannels(db)
		if err != nil {
			http.Error(
				w,
				"Impossible de lire les chaînes",
				http.StatusInternalServerError,
			)
			log.Printf(
				"Erreur lecture SQLite : %v",
				err,
			)
			return
		}

		// ------------------------------------------------------------
		// Vue destinée au template
		// ------------------------------------------------------------

		type ChannelView struct {
			database.Channel
			ColorLogoURL      string
			MonochromeLogoURL string
		}

		type PageData struct {
			Channels []ChannelView

			Enabled map[int]bool

			TotalCount    int
			EnabledCount  int
			BaseCount     int
			AlaCarteCount int
			NewCount      int

			ProviderName string
		}

		enabled := make(map[int]bool)

		channelViews := make(
			[]ChannelView,
			0,
			len(channels),
		)

		baseCount := 0
		newCount := 0
		enabledCount := 0

		for _, channel := range channels {

			if channel.Enabled {
				enabled[channel.ID] = true
				enabledCount++
			}

			if channel.IsBase {
				baseCount++
			}

			if channel.IsNew {
				newCount++
			}

			channelViews = append(
				channelViews,
				ChannelView{
					Channel: channel,

					ColorLogoURL: fmt.Sprintf(
						"/logos/%d/color.png",
						channel.ID,
					),

					MonochromeLogoURL: fmt.Sprintf(
						"/logos/%d/mono.png",
						channel.ID,
					),
				},
			)
		}

		data := PageData{
			Channels:      channelViews,
			Enabled:       enabled,
			TotalCount:    len(channels),
			EnabledCount:  enabledCount,
			BaseCount:     baseCount,
			AlaCarteCount: len(channels) - baseCount,
			NewCount:      newCount,
			ProviderName:  strings.ToUpper(cfg.Provider),
		}

		if err := tmpl.Execute(w, data); err != nil {
			log.Printf(
				"Erreur template : %v",
				err,
			)
		}
	})

	// ------------------------------------------------------------
	// Guides XMLTV
	// ------------------------------------------------------------

	epgHandler := func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(
				w,
				"Méthode non autorisée",
				http.StatusMethodNotAllowed,
			)
			return
		}

		query := r.URL.Query()

		// ============================================================
		// /epg.xml sans paramètres :
		// servir directement le fichier quotidien pré-généré.
		//
		// Cela constitue l'URL principale destinée aux clients IPTV.
		// ============================================================

		if len(query) == 0 {

			info, err := os.Stat(cfg.EPGXMLPath)
			if err != nil {
				if os.IsNotExist(err) {
					http.Error(
						w,
						"Guide XMLTV indisponible",
						http.StatusNotFound,
					)
					return
				}

				log.Printf(
					"Erreur accès XMLTV %s : %v",
					cfg.EPGXMLPath,
					err,
				)

				http.Error(
					w,
					"Guide XMLTV indisponible",
					http.StatusInternalServerError,
				)
				return
			}

			if info.IsDir() {
				http.Error(
					w,
					"Guide XMLTV invalide",
					http.StatusInternalServerError,
				)
				return
			}

			w.Header().Set(
				"Content-Type",
				"application/xml; charset=utf-8",
			)

			http.ServeFile(w, r, cfg.EPGXMLPath)
			return
		}

		// ============================================================
		// Génération dynamique
		//
		// /epg.xml?days=1
		// /epg.xml?days=7
		// /epg.xml?days=14&logo=color
		// /epg.xml?days=7&channels=303125,303117
		//
		// Le fichier /data/epg.xml n'est jamais modifié.
		// ============================================================

		days := 14

		if value := strings.TrimSpace(query.Get("days")); value != "" {

			parsed, err := strconv.Atoi(value)
			if err != nil {
				http.Error(
					w,
					"Paramètre days invalide",
					http.StatusBadRequest,
				)
				return
			}

			days = parsed
		}

		if days < 1 || days > 14 {
			http.Error(
				w,
				"Le paramètre days doit être compris entre 1 et 14",
				http.StatusBadRequest,
			)
			return
		}

		channels, err := parseEPGChannels(
			query.Get("channels"),
		)
		if err != nil {
			http.Error(
				w,
				err.Error(),
				http.StatusBadRequest,
			)
			return
		}

		logo := strings.TrimSpace(query.Get("logo"))

		if logo != "" &&
			logo != "color" &&
			logo != "mono" {

			http.Error(
				w,
				"Le paramètre logo doit être color ou mono",
				http.StatusBadRequest,
			)
			return
		}

		var buffer bytes.Buffer

		result, err := generator.GenerateToWriter(
			db,
			generator.Options{
				Channels: channels,
				Days:     days,
				Logo:     logo,
				BaseURL:  cfg.EPGBaseURL,
			},
			&buffer,
		)

		if err != nil {
			log.Printf(
				"Erreur génération XMLTV dynamique : %v",
				err,
			)

			http.Error(
				w,
				"Impossible de générer le guide XMLTV",
				http.StatusInternalServerError,
			)
			return
		}

		log.Printf(
			"Guide XMLTV dynamique : %d chaînes, %d événements, days=%d, channels=%q, logo=%q",
			result.ChannelCount,
			result.EventCount,
			days,
			query.Get("channels"),
			logo,
		)

		w.Header().Set(
			"Content-Type",
			"application/xml; charset=utf-8",
		)

		w.Header().Set(
			"Content-Length",
			strconv.Itoa(buffer.Len()),
		)

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		if _, err := w.Write(buffer.Bytes()); err != nil {
			log.Printf(
				"Erreur envoi XMLTV dynamique : %v",
				err,
			)
		}
	}

	// URL XMLTV principale.
	http.HandleFunc("/epg.xml", epgHandler)

	// Alias dynamique conservé pour compatibilité et diagnostics.
	http.HandleFunc("/epg", epgHandler)

	// ------------------------------------------------------------
	// Serveur
	// ------------------------------------------------------------

	addr := cfg.Host + ":" + cfg.Port

	log.Printf(
		"EPG Web démarré sur http://%s",
		addr,
	)

	channels, err := database.GetAllChannels(db)
	if err != nil {
		log.Fatalf(
			"Impossible de compter les chaînes : %v",
			err,
		)
	}

	log.Printf(
		"Chaînes disponibles dans SQLite : %d",
		len(channels),
	)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
