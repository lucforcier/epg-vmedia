package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(filename string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return nil, fmt.Errorf("création du répertoire de la base: %w", err)
	}

	// Les foreign keys doivent être activées sur chaque connexion
	// SQLite. Le driver modernc.org/sqlite applique ce paramètre
	// lors de l'ouverture de chaque connexion.
	dsn := filename + "?_foreign_keys=on"

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("ouverture de la base SQLite: %w", err)
	}

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("connexion à SQLite: %w", err)
	}

	return database, nil
}

// Initialize crée le schéma SQLite définitif.
//
// Pour une nouvelle base, le schéma est créé directement.
// Pour une base existante, une migration est effectuée si nécessaire.
func Initialize(database *sql.DB) error {
	exists, err := tableExists(database, "channels")
	if err != nil {
		return fmt.Errorf("vérification table channels: %w", err)
	}

	if !exists {
		if err := createSchema(database); err != nil {
			return err
		}
		return nil
	}

	if err := migrateSchema(database); err != nil {
		return err
	}

	return nil
}

// ------------------------------------------------------------
// Création du schéma définitif
// ------------------------------------------------------------

func createSchema(database *sql.DB) error {
	const schema = `
CREATE TABLE channels (
    id INTEGER PRIMARY KEY,
    access_num INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL DEFAULT '',
    callsign TEXT NOT NULL DEFAULT '',
    provider_channel_id TEXT NOT NULL DEFAULT '',
    visible INTEGER NOT NULL DEFAULT 0,

    enabled INTEGER NOT NULL DEFAULT 0,

    logo_preference TEXT NOT NULL DEFAULT 'color',

    color_logo_id INTEGER NOT NULL DEFAULT 0,
    mono_logo_id INTEGER NOT NULL DEFAULT 0,

    first_seen TEXT NOT NULL DEFAULT '',
    last_seen TEXT NOT NULL DEFAULT '',

    is_new INTEGER NOT NULL DEFAULT 0,
    is_base INTEGER NOT NULL DEFAULT 0
);
`

	if _, err := database.Exec(schema); err != nil {
		return fmt.Errorf("création du schéma SQLite: %w", err)
	}

	return nil
}

// ------------------------------------------------------------
// Migration d'une ancienne base
// ------------------------------------------------------------

func migrateSchema(database *sql.DB) error {
	columns, err := getColumns(database)
	if err != nil {
		return fmt.Errorf("lecture du schéma SQLite: %w", err)
	}

	// Si le schéma définitif est déjà présent, rien à faire.
	if hasColumn(columns, "is_new") &&
		hasColumn(columns, "is_base") &&
		!hasColumn(columns, "channel_type") &&
		!hasColumn(columns, "color_logo_path") &&
		!hasColumn(columns, "mono_logo_path") {
		return nil
	}

	// --------------------------------------------------------
	// Migration transactionnelle
	// --------------------------------------------------------

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("début migration SQLite: %w", err)
	}

	defer tx.Rollback()

	const newTable = `
CREATE TABLE channels_new (
    id INTEGER PRIMARY KEY,
    access_num INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL DEFAULT '',
    callsign TEXT NOT NULL DEFAULT '',
    provider_channel_id TEXT NOT NULL DEFAULT '',
    visible INTEGER NOT NULL DEFAULT 0,

    enabled INTEGER NOT NULL DEFAULT 0,

    logo_preference TEXT NOT NULL DEFAULT 'color',

    color_logo_id INTEGER NOT NULL DEFAULT 0,
    mono_logo_id INTEGER NOT NULL DEFAULT 0,

    first_seen TEXT NOT NULL DEFAULT '',
    last_seen TEXT NOT NULL DEFAULT '',

    is_new INTEGER NOT NULL DEFAULT 0,
    is_base INTEGER NOT NULL DEFAULT 0
);
`

	if _, err := tx.Exec(newTable); err != nil {
		return fmt.Errorf("création table channels_new: %w", err)
	}

	isNewExpression := "0"
	if hasColumn(columns, "is_new") {
		isNewExpression = "is_new"
	}

	isBaseExpression := "0"
	if hasColumn(columns, "is_base") {
		isBaseExpression = "is_base"
	}

	const insertPrefix = `
INSERT INTO channels_new (
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
SELECT
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
`

	insertQuery := insertPrefix +
		isNewExpression + ",\n    " +
		isBaseExpression + `
FROM channels;
`

	if _, err := tx.Exec(insertQuery); err != nil {
		return fmt.Errorf("copie des chaînes vers le nouveau schéma: %w", err)
	}

	if _, err := tx.Exec(`DROP TABLE channels`); err != nil {
		return fmt.Errorf("suppression ancien schéma channels: %w", err)
	}

	if _, err := tx.Exec(
		`ALTER TABLE channels_new RENAME TO channels`,
	); err != nil {
		return fmt.Errorf("renommage nouveau schéma channels: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("validation migration SQLite: %w", err)
	}

	return nil
}

// ------------------------------------------------------------
// Utilitaires schéma
// ------------------------------------------------------------

func tableExists(database *sql.DB, tableName string) (bool, error) {
	var count int

	err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master
         WHERE type = 'table' AND name = ?`,
		tableName,
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func getColumns(database *sql.DB) (map[string]bool, error) {
	rows, err := database.Query(`PRAGMA table_info(channels)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)

	for rows.Next() {
		var (
			cid       int
			name      string
			columnTyp string
			notNull   int
			defaultV  sql.NullString
			primary   int
		)

		if err := rows.Scan(
			&cid,
			&name,
			&columnTyp,
			&notNull,
			&defaultV,
			&primary,
		); err != nil {
			return nil, err
		}

		columns[name] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return columns, nil
}

func hasColumn(columns map[string]bool, name string) bool {
	return columns[name]
}

// ------------------------------------------------------------
// Modèle Channel
// ------------------------------------------------------------

type Channel struct {
	ID                int
	AccessNum         int
	Name              string
	CallSign          string
	ProviderChannelID string
	Visible           bool
	Enabled           bool
	LogoPreference    string
	ColorLogoID       int
	MonoLogoID        int
	FirstSeen         string
	LastSeen          string
	IsNew             bool
	IsBase            bool
}

// ------------------------------------------------------------
// Lecture
// ------------------------------------------------------------

func GetAllChannels(database *sql.DB) ([]Channel, error) {
	const query = `
SELECT
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
FROM channels
ORDER BY
    is_new DESC,
    access_num,
    id
`

	return queryChannels(database, query, "lecture des chaînes")
}

// ------------------------------------------------------------
// Sélection EPG
// ------------------------------------------------------------

// GetEnabledChannels retourne toutes les chaînes activées dans SQLite.
//
// Cette fonction constitue la sélection de base du guide EPG.
// Aucune chaîne disabled n'est retournée.
func GetEnabledChannels(database *sql.DB) ([]Channel, error) {
	const query = `
SELECT
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
FROM channels
WHERE enabled = 1
ORDER BY
    is_new DESC,
    access_num,
    id
`

	return queryChannels(
		database,
		query,
		"lecture des chaînes activées",
	)
}

// GetEPGChannels retourne les chaînes qui doivent être publiées
// dans un guide XMLTV.
//
// Règle fondamentale:
//
//   - requestedIDs vide : toutes les chaînes enabled.
//   - requestedIDs non vide : intersection entre requestedIDs et
//     les chaînes enabled.
//
// Une chaîne demandée mais disabled ou inexistante est ignorée.
// Si aucune chaîne ne subsiste après l'intersection, une erreur
// est retournée.
//
// Cette fonction ne modifie jamais SQLite et ne contacte jamais VMedia.
func GetEPGChannels(
	database *sql.DB,
	requestedIDs []int,
) ([]Channel, error) {

	enabledChannels, err := GetEnabledChannels(database)
	if err != nil {
		return nil, err
	}

	// Aucun filtre explicite :
	// le guide porte sur toutes les chaînes enabled.
	if len(requestedIDs) == 0 {
		if len(enabledChannels) == 0 {
			return nil, fmt.Errorf(
				"aucune chaîne enabled dans SQLite",
			)
		}

		return enabledChannels, nil
	}

	// Éliminer les doublons tout en conservant la demande.
	requested := make(map[int]struct{}, len(requestedIDs))

	for _, id := range requestedIDs {
		if id <= 0 {
			return nil, fmt.Errorf(
				"identifiant de chaîne invalide : %d",
				id,
			)
		}

		requested[id] = struct{}{}
	}

	// L'ordre du guide reste celui défini par SQLite :
	// is_new, access_num, id.
	filtered := make([]Channel, 0, len(requested))

	for _, channel := range enabledChannels {
		if _, ok := requested[channel.ID]; ok {
			filtered = append(filtered, channel)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf(
			"aucune des chaînes demandées n'est enabled",
		)
	}

	return filtered, nil
}

// FilterEnabledChannels applique l'intersection entre une liste
// de chaînes et une liste d'IDs demandés.
//
// Cette fonction est conservée comme utilitaire générique.
// Elle ne génère aucune erreur si l'intersection est vide.
func FilterEnabledChannels(
	channels []Channel,
	requestedIDs []int,
) []Channel {
	if len(requestedIDs) == 0 {
		return channels
	}

	requested := make(map[int]struct{}, len(requestedIDs))

	for _, id := range requestedIDs {
		requested[id] = struct{}{}
	}

	filtered := make([]Channel, 0, len(requestedIDs))

	for _, channel := range channels {
		if _, ok := requested[channel.ID]; ok {
			filtered = append(filtered, channel)
		}
	}

	return filtered
}

// ------------------------------------------------------------
// Lecture générique des chaînes
// ------------------------------------------------------------

func queryChannels(
	database *sql.DB,
	query string,
	context string,
) ([]Channel, error) {

	rows, err := database.Query(query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", context, err)
	}
	defer rows.Close()

	var channels []Channel

	for rows.Next() {
		var (
			channel Channel
			visible int
			enabled int
			isNew   int
			isBase  int
		)

		if err := rows.Scan(
			&channel.ID,
			&channel.AccessNum,
			&channel.Name,
			&channel.CallSign,
			&channel.ProviderChannelID,
			&visible,
			&enabled,
			&channel.LogoPreference,
			&channel.ColorLogoID,
			&channel.MonoLogoID,
			&channel.FirstSeen,
			&channel.LastSeen,
			&isNew,
			&isBase,
		); err != nil {
			return nil, fmt.Errorf(
				"%s: lecture d'une chaîne: %w",
				context,
				err,
			)
		}

		channel.Visible = visible != 0
		channel.Enabled = enabled != 0
		channel.IsNew = isNew != 0
		channel.IsBase = isBase != 0

		channels = append(channels, channel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"%s: parcours des chaînes: %w",
			context,
			err,
		)
	}

	return channels, nil
}

// ------------------------------------------------------------
// Modification enabled
// ------------------------------------------------------------

func SetChannelEnabled(
	database *sql.DB,
	channelID int,
	enabled bool,
) error {
	value := 0
	if enabled {
		value = 1
	}

	_, err := database.Exec(
		`UPDATE channels SET enabled = ? WHERE id = ?`,
		value,
		channelID,
	)

	if err != nil {
		return fmt.Errorf(
			"mise à jour enabled chaîne %d: %w",
			channelID,
			err,
		)
	}

	return nil
}

// ------------------------------------------------------------
// Préférence logo
// ------------------------------------------------------------

func SetLogoPreference(
	database *sql.DB,
	channelID int,
	preference string,
) error {
	if preference != "color" && preference != "mono" {
		return fmt.Errorf(
			"préférence de logo invalide : %q",
			preference,
		)
	}

	_, err := database.Exec(
		`UPDATE channels
         SET logo_preference = ?
         WHERE id = ?`,
		preference,
		channelID,
	)

	if err != nil {
		return fmt.Errorf(
			"mise à jour préférence logo chaîne %d: %w",
			channelID,
			err,
		)
	}

	return nil
}

// ------------------------------------------------------------
// Chaîne de base
// ------------------------------------------------------------

func SetChannelBase(
	database *sql.DB,
	channelID int,
	isBase bool,
) error {
	value := 0
	if isBase {
		value = 1
	}

	_, err := database.Exec(
		`UPDATE channels
         SET is_base = ?
         WHERE id = ?`,
		value,
		channelID,
	)

	if err != nil {
		return fmt.Errorf(
			"mise à jour chaîne de base %d: %w",
			channelID,
			err,
		)
	}

	return nil
}

// ------------------------------------------------------------
// Nouvelle chaîne traitée
// ------------------------------------------------------------

func MarkChannelProcessed(
	database *sql.DB,
	channelID int,
) error {
	_, err := database.Exec(
		`UPDATE channels
         SET is_new = 0
         WHERE id = ?`,
		channelID,
	)

	if err != nil {
		return fmt.Errorf(
			"marquage chaîne traitée %d: %w",
			channelID,
			err,
		)
	}

	return nil
}

// ------------------------------------------------------------
// Utilitaires
// ------------------------------------------------------------

// normalizeIDs retourne une liste d'IDs valides, uniques et triés.
//
// Cette fonction n'est pas utilisée actuellement par GetEPGChannels,
// mais elle sera utile lorsque les paramètres HTTP channels= seront
// introduits.
func normalizeIDs(ids []int) ([]int, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	seen := make(map[int]struct{}, len(ids))
	result := make([]int, 0, len(ids))

	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf(
				"identifiant de chaîne invalide : %d",
				id,
			)
		}

		if _, exists := seen[id]; exists {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	sort.Ints(result)

	return result, nil
}

// parseChannelIDs convertit une liste textuelle d'IDs séparés par
// des virgules en []int.
//
// Cette fonction sera utilisée par le futur handler HTTP.
func parseChannelIDs(value string) ([]int, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	ids := make([]int, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			return nil, fmt.Errorf(
				"identifiant de chaîne vide dans %q",
				value,
			)
		}

		var id int

		if _, err := fmt.Sscanf(part, "%d", &id); err != nil {
			return nil, fmt.Errorf(
				"identifiant de chaîne invalide %q",
				part,
			)
		}

		ids = append(ids, id)
	}

	return normalizeIDs(ids)
}
