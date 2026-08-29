package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/database"
	"epg-vmedia/internal/generator"
)

func parseChannels(value string) ([]int, error) {
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
				"ID de chaîne vide dans -channels",
			)
		}

		id, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf(
				"ID de chaîne invalide %q",
				part,
			)
		}

		if id <= 0 {
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

	days := flag.Int(
		"days",
		14,
		"Nombre de jours de guide à générer",
	)

	output := flag.String(
		"output",
		cfg.EPGXMLPath,
		"Fichier XMLTV de sortie",
	)

	channelsValue := flag.String(
		"channels",
		"",
		"Liste des IDs de chaînes séparés par des virgules",
	)

	logo := flag.String(
		"logo",
		"",
		"Préférence de logo : color ou mono",
	)

	flag.Parse()

	if *days < 1 || *days > 14 {
		log.Fatalf(
			"Le nombre de jours doit être compris entre 1 et 14",
		)
	}

	if *output == "" {
		log.Fatalf(
			"Le fichier de sortie XMLTV est vide",
		)
	}

	channels, err := parseChannels(*channelsValue)
	if err != nil {
		log.Fatalf("Erreur paramètre -channels : %v", err)
	}

	if *logo != "" &&
		*logo != "color" &&
		*logo != "mono" {
		log.Fatalf(
			"Préférence de logo invalide : %q",
			*logo,
		)
	}

	log.Printf("===== GÉNÉRATION XMLTV =====")
	log.Printf(
		"Base SQLite : %s",
		filepath.Join(cfg.DataDir, "epg.db"),
	)
	log.Printf("Sortie : %s", *output)
	log.Printf("Nombre de jours : %d", *days)
	log.Printf("EPG_BASE_URL : %s", cfg.EPGBaseURL)

	if *channelsValue == "" {
		log.Printf("Chaînes demandées : toutes les chaînes enabled")
	} else {
		log.Printf(
			"Chaînes demandées : %s",
			*channelsValue,
		)
	}

	if *logo == "" {
		log.Printf(
			"Logo : préférence de chaque chaîne",
		)
	} else {
		log.Printf("Logo : %s", *logo)
	}

	// ------------------------------------------------------------
	// SQLite
	// ------------------------------------------------------------

	databaseFile := filepath.Join(
		cfg.DataDir,
		"epg.db",
	)

	db, err := database.Open(databaseFile)
	if err != nil {
		log.Fatalf(
			"Erreur ouverture SQLite : %v",
			err,
		)
	}
	defer db.Close()

	if err := database.Initialize(db); err != nil {
		log.Fatalf(
			"Erreur initialisation SQLite : %v",
			err,
		)
	}

	// ------------------------------------------------------------
	// Génération
	// ------------------------------------------------------------

	result, err := generator.Generate(
		db,
		generator.Options{
			Channels: channels,
			Days:     *days,
			Logo:     *logo,
			BaseURL:  cfg.EPGBaseURL,
			Output:   *output,
		},
	)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// ------------------------------------------------------------
	// Résultat
	// ------------------------------------------------------------

	log.Printf(
		"Chaînes XMLTV : %d",
		result.ChannelCount,
	)

	log.Printf(
		"Événements : %d",
		result.EventCount,
	)

	log.Printf(
		"XMLTV généré avec succès : %s",
		result.OutputPath,
	)

	fmt.Printf(
		"OK: %s (%d chaînes, %d événements)\n",
		result.OutputPath,
		result.ChannelCount,
		result.EventCount,
	)
}
