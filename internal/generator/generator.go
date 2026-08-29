package generator

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"epg-vmedia/internal/database"
	"epg-vmedia/internal/xmltv"
)

// Options définit les paramètres de génération d'un guide XMLTV.
type Options struct {
	// Channels contient les IDs des chaînes demandées.
	//
	// Une liste vide signifie : toutes les chaînes enabled.
	Channels []int

	// Days est le nombre de jours de guide à produire.
	Days int

	// Logo impose éventuellement une préférence de logo globale.
	//
	// Valeurs acceptées :
	//   ""      = utiliser la préférence de chaque chaîne
	//   "color" = imposer les logos couleur
	//   "mono"  = imposer les logos monochromes
	Logo string

	// BaseURL est l'URL publique utilisée par XMLTV
	// pour les logos et les images.
	BaseURL string

	// Output est le chemin du fichier XMLTV de sortie.
	//
	// Utilisé uniquement par Generate().
	Output string
}

// Result contient le résultat d'une génération réussie.
type Result struct {
	OutputPath   string
	ChannelCount int
	EventCount   int
}

// validateOptions valide les paramètres communs aux deux modes
// de génération.
func validateOptions(options Options) error {

	if options.Days < 1 || options.Days > 14 {
		return fmt.Errorf(
			"le nombre de jours doit être compris entre 1 et 14",
		)
	}

	if options.Logo != "" &&
		options.Logo != "color" &&
		options.Logo != "mono" {
		return fmt.Errorf(
			"préférence de logo invalide : %q",
			options.Logo,
		)
	}

	return nil
}

// prepare construit les données XMLTV à partir de SQLite.
//
// Aucune écriture de fichier ni aucun accès réseau n'est effectué ici.
func prepare(
	db *sql.DB,
	options Options,
) ([]xmltv.Channel, []xmltv.Event, error) {

	if err := validateOptions(options); err != nil {
		return nil, nil, err
	}

	// ------------------------------------------------------------
	// Chaînes enabled
	// ------------------------------------------------------------

	dbChannels, err := database.GetEnabledChannels(db)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"lecture des chaînes activées : %w",
			err,
		)
	}

	if len(dbChannels) == 0 {
		return nil, nil, fmt.Errorf(
			"aucune chaîne enabled dans SQLite",
		)
	}

	// ------------------------------------------------------------
	// Intersection requested / enabled
	// ------------------------------------------------------------

	selectedChannels := database.FilterEnabledChannels(
		dbChannels,
		options.Channels,
	)

	if len(options.Channels) > 0 && len(selectedChannels) == 0 {
		return nil, nil, fmt.Errorf(
			"aucune des chaînes demandées n'est enabled dans SQLite",
		)
	}

	channelIDs := make([]int, 0, len(selectedChannels))
	xmlChannels := make([]xmltv.Channel, 0, len(selectedChannels))

	for _, channel := range selectedChannels {
		channelIDs = append(channelIDs, channel.ID)

		xmlChannels = append(xmlChannels, xmltv.Channel{
			ID:             channel.ID,
			AccessNum:      channel.AccessNum,
			Name:           channel.Name,
			CallSign:       channel.CallSign,
			LogoPreference: channel.LogoPreference,
			ColorLogoID:    int64(channel.ColorLogoID),
			MonoLogoID:     int64(channel.MonoLogoID),
		})
	}

	// ------------------------------------------------------------
	// Fenêtre temporelle
	// ------------------------------------------------------------

	now := time.Now().UTC()

	startUTC := now.Format(time.RFC3339)
	endUTC := now.
		Add(time.Duration(options.Days) * 24 * time.Hour).
		Format(time.RFC3339)

	// ------------------------------------------------------------
	// Événements
	// ------------------------------------------------------------

	dbEvents, err := database.GetEPGEvents(
		db,
		channelIDs,
		startUTC,
		endUTC,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"lecture des événements : %w",
			err,
		)
	}

	xmlEvents := make([]xmltv.Event, 0, len(dbEvents))

	for _, event := range dbEvents {
		xmlEvent := xmltv.Event{
			EventID:          event.EventID,
			ChannelID:        event.ChannelID,
			ChannelName:      event.ChannelName,
			ChannelAccessNum: event.ChannelAccessNum,

			Title:            event.Title,
			Description:      event.Description,
			ShortDescription: event.ShortDescription,

			StartTimeUTC:    event.StartTimeUTC,
			DurationSeconds: event.DurationSeconds,
		}

		if event.ProgramImageID.Valid {
			xmlEvent.ProgramImageID = event.ProgramImageID.Int64
			xmlEvent.HasProgramImage = true

			if event.ProgramImageType.Valid {
				xmlEvent.ProgramImageType =
					int(event.ProgramImageType.Int64)
			}
		}

		xmlEvents = append(xmlEvents, xmlEvent)
	}

	return xmlChannels, xmlEvents, nil
}

// GenerateToWriter génère un document XMLTV directement dans w.
//
// Cette fonction est utilisée notamment pour les réponses HTTP
// générées à la demande.
//
// Elle ne crée aucun fichier temporaire et ne fait aucun accès réseau.
func GenerateToWriter(
	db *sql.DB,
	options Options,
	w io.Writer,
) (Result, error) {

	if err := validateOptions(options); err != nil {
		return Result{}, err
	}

	if w == nil {
		return Result{}, fmt.Errorf(
			"writer XMLTV nil",
		)
	}

	xmlChannels, xmlEvents, err := prepare(
		db,
		options,
	)
	if err != nil {
		return Result{}, err
	}

	if err := xmltv.Write(
		w,
		xmlChannels,
		xmlEvents,
		options.BaseURL,
		options.Logo,
	); err != nil {
		return Result{}, fmt.Errorf(
			"génération XMLTV : %w",
			err,
		)
	}

	return Result{
		ChannelCount: len(xmlChannels),
		EventCount:   len(xmlEvents),
	}, nil
}

// Generate produit le fichier XMLTV selon les options fournies.
//
// Les données sont lues exclusivement depuis SQLite.
// La génération est effectuée dans un fichier temporaire,
// puis le fichier final est remplacé atomiquement.
//
// En cas d'erreur, le fichier XMLTV existant n'est pas remplacé.
func Generate(
	db *sql.DB,
	options Options,
) (Result, error) {

	if err := validateOptions(options); err != nil {
		return Result{}, err
	}

	if options.Output == "" {
		return Result{}, fmt.Errorf(
			"le fichier de sortie XMLTV est vide",
		)
	}

	xmlChannels, xmlEvents, err := prepare(
		db,
		options,
	)
	if err != nil {
		return Result{}, err
	}

	// ------------------------------------------------------------
	// Répertoire de sortie
	// ------------------------------------------------------------

	outputPath := options.Output
	outputDir := filepath.Dir(outputPath)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return Result{}, fmt.Errorf(
			"impossible de créer le répertoire de sortie %s: %w",
			outputDir,
			err,
		)
	}

	// ------------------------------------------------------------
	// Fichier temporaire
	// ------------------------------------------------------------

	tempFile, err := os.CreateTemp(
		outputDir,
		".epg.xml.*.tmp",
	)
	if err != nil {
		return Result{}, fmt.Errorf(
			"impossible de créer le fichier temporaire : %w",
			err,
		)
	}

	tempPath := tempFile.Name()
	success := false

	defer func() {
		_ = tempFile.Close()

		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	// ------------------------------------------------------------
	// Génération XMLTV
	// ------------------------------------------------------------

	if err := xmltv.Write(
		tempFile,
		xmlChannels,
		xmlEvents,
		options.BaseURL,
		options.Logo,
	); err != nil {
		return Result{}, fmt.Errorf(
			"génération XMLTV : %w",
			err,
		)
	}

	// ------------------------------------------------------------
	// Synchronisation
	// ------------------------------------------------------------

	if err := tempFile.Sync(); err != nil {
		return Result{}, fmt.Errorf(
			"synchronisation du fichier XMLTV : %w",
			err,
		)
	}

	if err := tempFile.Close(); err != nil {
		return Result{}, fmt.Errorf(
			"fermeture du fichier XMLTV temporaire : %w",
			err,
		)
	}

	// ------------------------------------------------------------
	// Remplacement atomique
	// ------------------------------------------------------------

	if err := os.Rename(tempPath, outputPath); err != nil {
		return Result{}, fmt.Errorf(
			"remplacement atomique de %s : %w",
			outputPath,
			err,
		)
	}

	success = true

	return Result{
		OutputPath:   outputPath,
		ChannelCount: len(xmlChannels),
		EventCount:   len(xmlEvents),
	}, nil
}
