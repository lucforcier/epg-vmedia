package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"epg-vmedia/internal/vmedia"
)

// UpsertContainer crée ou met à jour un container VMedia.
func UpsertContainer(
	tx *sql.Tx,
	item vmedia.GuideItem,
	sourceJSON string,
) (int64, error) {

	if item.ContainerID == 0 {
		return 0, nil
	}

	_, err := tx.Exec(`
INSERT INTO containers (
	vmedia_container_id,
	name,
	source_json
)
VALUES (?, ?, ?)
ON CONFLICT(vmedia_container_id)
DO UPDATE SET
	name = excluded.name,
	source_json = excluded.source_json
`,
		item.ContainerID,
		item.Name,
		sourceJSON,
	)

	if err != nil {
		return 0, fmt.Errorf(
			"upsert container %d: %w",
			item.ContainerID,
			err,
		)
	}

	var id int64

	err = tx.QueryRow(`
SELECT id
FROM containers
WHERE vmedia_container_id = ?
`,
		item.ContainerID,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf(
			"lecture container %d: %w",
			item.ContainerID,
			err,
		)
	}

	return id, nil
}

// UpsertProgram crée ou met à jour un programme VMedia.
func UpsertProgram(
	tx *sql.Tx,
	item vmedia.GuideItem,
	containerID sql.NullInt64,
	sourceJSON string,
) (int64, error) {

	_, err := tx.Exec(`
INSERT INTO programs (
	vmedia_program_id,
	provider_program_id,
	container_id,
	name,
	description,
	short_description,
	source_json
)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(vmedia_program_id)
DO UPDATE SET
	provider_program_id = excluded.provider_program_id,
	container_id = excluded.container_id,
	name = excluded.name,
	description = excluded.description,
	short_description = excluded.short_description,
	source_json = excluded.source_json
`,
		item.ProgramID,
		item.ProviderProgramID,
		containerID,
		item.Name,
		item.Description,
		item.ShortDescription,
		sourceJSON,
	)

	if err != nil {
		return 0, fmt.Errorf(
			"upsert programme %d: %w",
			item.ProgramID,
			err,
		)
	}

	var id int64

	err = tx.QueryRow(`
SELECT id
FROM programs
WHERE vmedia_program_id = ?
`,
		item.ProgramID,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf(
			"lecture programme %d: %w",
			item.ProgramID,
			err,
		)
	}

	return id, nil
}

// UpsertImage crée ou retrouve une image VMedia.
func UpsertImage(
	tx *sql.Tx,
	image vmedia.GuideImage,
) (int64, error) {

	_, err := tx.Exec(`
INSERT INTO images (
	vmedia_image_id,
	image_type
)
VALUES (?, ?)
ON CONFLICT(vmedia_image_id, image_type)
DO NOTHING
`,
		image.ID,
		image.Type,
	)

	if err != nil {
		return 0, fmt.Errorf(
			"upsert image %d type %d: %w",
			image.ID,
			image.Type,
			err,
		)
	}

	var id int64

	err = tx.QueryRow(`
SELECT id
FROM images
WHERE vmedia_image_id = ?
  AND image_type = ?
`,
		image.ID,
		image.Type,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf(
			"lecture image %d type %d: %w",
			image.ID,
			image.Type,
			err,
		)
	}

	return id, nil
}

// LinkProgramImage associe une image à un programme.
func LinkProgramImage(
	tx *sql.Tx,
	programID int64,
	imageID int64,
) error {

	_, err := tx.Exec(`
INSERT OR IGNORE INTO program_images (
	program_id,
	image_id
)
VALUES (?, ?)
`,
		programID,
		imageID,
	)

	return err
}

// LinkSeriesImage associe une image à un container.
func LinkSeriesImage(
	tx *sql.Tx,
	containerID int64,
	imageID int64,
) error {

	_, err := tx.Exec(`
INSERT OR IGNORE INTO series_images (
	container_id,
	image_id
)
VALUES (?, ?)
`,
		containerID,
		imageID,
	)

	return err
}

// InsertEvent insère ou met à jour une occurrence du guide.
//
// IMPORTANT : VMedia fournit item.Length en MINUTES.
// La base de données stocke duration_seconds en SECONDES.
// La conversion minutes -> secondes est donc effectuée ici.
//
// IMPORTANT : vmedia_event_id constitue l'identité de l'occurrence
// VMedia. Lorsqu'un événement existe déjà, il est mis à jour au lieu
// d'être inséré une seconde fois.
//
// Cette fonction suppose que events.vmedia_event_id possède une
// contrainte UNIQUE dans le schéma SQLite.
func InsertEvent(
	tx *sql.Tx,
	item vmedia.GuideItem,
	channelID int,
	programID int64,
	containerID sql.NullInt64,
	startTimeUTC string,
	sourceJSON string,
) error {

	_, err := tx.Exec(`
INSERT INTO events (
	vmedia_event_id,
	channel_id,
	program_id,
	container_id,
	provider_program_id,
	start_time_utc,
	duration_seconds,
	source_json
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(vmedia_event_id)
DO UPDATE SET
	channel_id = excluded.channel_id,
	program_id = excluded.program_id,
	container_id = excluded.container_id,
	provider_program_id = excluded.provider_program_id,
	start_time_utc = excluded.start_time_utc,
	duration_seconds = excluded.duration_seconds,
	source_json = excluded.source_json
`,
		item.ID,
		channelID,
		programID,
		containerID,
		item.ProviderProgramID,
		startTimeUTC,
		item.Length*60,
		sourceJSON,
	)

	if err != nil {
		return fmt.Errorf(
			"upsert événement %d: %w",
			item.ID,
			err,
		)
	}

	return nil
}

// MarshalGuideItem conserve une représentation JSON du programme.
func MarshalGuideItem(item vmedia.GuideItem) (string, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return "", fmt.Errorf(
			"sérialisation programme %d: %w",
			item.ID,
			err,
		)
	}

	return string(data), nil
}

// ------------------------------------------------------------
// Lecture EPG pour génération XMLTV
// ------------------------------------------------------------

type EPGEvent struct {
	EventID     int64
	ChannelID   int
	ProgramID   int64
	ContainerID sql.NullInt64

	ChannelName      string
	ChannelAccessNum int

	Title            string
	Description      string
	ShortDescription string

	StartTimeUTC    string
	DurationSeconds int

	ProgramImageID   sql.NullInt64
	ProgramImageType sql.NullInt64
}

// GetEPGEvents retourne les événements d'un ensemble de chaînes
// pour une période donnée.
//
// Les dates startUTC/endUTC doivent être fournies en UTC,
// au format ISO 8601 utilisé par SQLite.
//
// Un seul événement est retourné par vmedia_event_id.
// Lorsqu'une image de type 28 existe pour le programme,
// elle est retournée avec l'événement.
func GetEPGEvents(
	database *sql.DB,
	channelIDs []int,
	startUTC string,
	endUTC string,
) ([]EPGEvent, error) {

	if len(channelIDs) == 0 {
		return []EPGEvent{}, nil
	}

	placeholders := make([]string, len(channelIDs))
	args := make([]interface{}, 0, len(channelIDs)+2)

	for i, channelID := range channelIDs {
		placeholders[i] = "?"
		args = append(args, channelID)
	}

	args = append(args, startUTC, endUTC)

	query := fmt.Sprintf(`
SELECT
	e.vmedia_event_id,
	e.channel_id,
	e.program_id,
	e.container_id,

	c.name,
	c.access_num,

	p.name,
	p.description,
	p.short_description,

	e.start_time_utc,
	e.duration_seconds,

	img.vmedia_image_id,
	img.image_type

FROM events e

JOIN channels c
	ON c.id = e.channel_id

JOIN programs p
	ON p.id = e.program_id

LEFT JOIN images img
	ON img.id = (
		SELECT pi.image_id
		FROM program_images pi
		JOIN images i
			ON i.id = pi.image_id
		WHERE pi.program_id = p.id
		  AND i.image_type = 28
		ORDER BY pi.image_id
		LIMIT 1
	)

WHERE e.channel_id IN (%s)
  AND e.start_time_utc >= ?
  AND e.start_time_utc < ?

ORDER BY
	e.channel_id,
	e.start_time_utc,
	e.id
`, strings.Join(placeholders, ","))

	rows, err := database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("lecture des événements EPG: %w", err)
	}
	defer rows.Close()

	var events []EPGEvent

	for rows.Next() {
		var event EPGEvent

		err := rows.Scan(
			&event.EventID,
			&event.ChannelID,
			&event.ProgramID,
			&event.ContainerID,

			&event.ChannelName,
			&event.ChannelAccessNum,

			&event.Title,
			&event.Description,
			&event.ShortDescription,

			&event.StartTimeUTC,
			&event.DurationSeconds,

			&event.ProgramImageID,
			&event.ProgramImageType,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"lecture d'un événement EPG: %w",
				err,
			)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"parcours des événements EPG: %w",
			err,
		)
	}

	return events, nil
}
