package database

import (
	"database/sql"
	"fmt"
)

// InitializeEPGSchema crée les tables nécessaires au stockage du guide
// VMedia.
//
// La table channels existe déjà et n'est pas recréée ici.
func InitializeEPGSchema(db *sql.DB) error {
	const schema = `
PRAGMA foreign_keys = ON;

-- ============================================================
-- CONTAINERS
-- ============================================================
--
-- Identité d'une série / émission récurrente VMedia.
--

CREATE TABLE IF NOT EXISTS containers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    vmedia_container_id INTEGER NOT NULL,

    name TEXT NOT NULL DEFAULT '',

    source_json TEXT NOT NULL DEFAULT '',

    UNIQUE (vmedia_container_id)
);

-- ============================================================
-- PROGRAMS
-- ============================================================
--
-- Identité éditoriale d'un programme VMedia.
--
-- Un programme peut appartenir à un container.
-- Un film ou contenu autonome peut avoir container_id NULL.
--

CREATE TABLE IF NOT EXISTS programs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    vmedia_program_id INTEGER NOT NULL,

    provider_program_id TEXT NOT NULL DEFAULT '',

    container_id INTEGER NULL,

    name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    short_description TEXT NOT NULL DEFAULT '',

    source_json TEXT NOT NULL DEFAULT '',

    FOREIGN KEY (container_id)
        REFERENCES containers(id)
        ON DELETE SET NULL,

    UNIQUE (vmedia_program_id)
);

-- ============================================================
-- EVENTS
-- ============================================================
--
-- Une occurrence dans la grille TV.
--
-- IMPORTANT :
-- un programme peut avoir plusieurs événements.
--
-- vmedia_event_id identifie de manière unique une occurrence
-- fournie par VMedia.
--

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    vmedia_event_id INTEGER NULL,

    channel_id INTEGER NOT NULL,

    program_id INTEGER NOT NULL,

    container_id INTEGER NULL,

    provider_program_id TEXT NOT NULL DEFAULT '',

    start_time_utc TEXT NOT NULL,

    duration_seconds INTEGER NOT NULL DEFAULT 0,

    source_json TEXT NOT NULL DEFAULT '',

    FOREIGN KEY (channel_id)
        REFERENCES channels(id)
        ON DELETE CASCADE,

    FOREIGN KEY (program_id)
        REFERENCES programs(id)
        ON DELETE CASCADE,

    FOREIGN KEY (container_id)
        REFERENCES containers(id)
        ON DELETE SET NULL,

    UNIQUE (vmedia_event_id)
);

-- ============================================================
-- IMAGES
-- ============================================================
--
-- Images VMedia identifiées par leur ID et leur type.
--
-- Type 28 = image principale
-- Type 29 = image secondaire
--
-- D'autres types peuvent exister et sont volontairement
-- conservés.
--

CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    vmedia_image_id INTEGER NOT NULL,

    image_type INTEGER NOT NULL,

    source_json TEXT NOT NULL DEFAULT '',

    UNIQUE (vmedia_image_id, image_type)
);

-- ============================================================
-- PROGRAM_IMAGES
-- ============================================================
--
-- Relation programme -> image
--

CREATE TABLE IF NOT EXISTS program_images (
    program_id INTEGER NOT NULL,

    image_id INTEGER NOT NULL,

    PRIMARY KEY (program_id, image_id),

    FOREIGN KEY (program_id)
        REFERENCES programs(id)
        ON DELETE CASCADE,

    FOREIGN KEY (image_id)
        REFERENCES images(id)
        ON DELETE CASCADE
);

-- ============================================================
-- SERIES_IMAGES
-- ============================================================
--
-- Relation container/série -> image
--

CREATE TABLE IF NOT EXISTS series_images (
    container_id INTEGER NOT NULL,

    image_id INTEGER NOT NULL,

    PRIMARY KEY (container_id, image_id),

    FOREIGN KEY (container_id)
        REFERENCES containers(id)
        ON DELETE CASCADE,

    FOREIGN KEY (image_id)
        REFERENCES images(id)
        ON DELETE CASCADE
);

-- ============================================================
-- INDEX
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_programs_container
    ON programs(container_id);

CREATE INDEX IF NOT EXISTS idx_programs_provider
    ON programs(provider_program_id);

CREATE INDEX IF NOT EXISTS idx_events_channel_start
    ON events(channel_id, start_time_utc);

CREATE INDEX IF NOT EXISTS idx_events_program
    ON events(program_id);

CREATE INDEX IF NOT EXISTS idx_events_container
    ON events(container_id);

CREATE INDEX IF NOT EXISTS idx_events_start
    ON events(start_time_utc);

CREATE INDEX IF NOT EXISTS idx_events_vmedia_event
    ON events(vmedia_event_id);

CREATE INDEX IF NOT EXISTS idx_images_vmedia
    ON images(vmedia_image_id);

CREATE INDEX IF NOT EXISTS idx_program_images_image
    ON program_images(image_id);

CREATE INDEX IF NOT EXISTS idx_series_images_image
    ON series_images(image_id);
`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("création du schéma EPG SQLite: %w", err)
	}

	return nil
}
