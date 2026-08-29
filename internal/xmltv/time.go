package xmltv

import (
	"fmt"
	"time"
)

// formatXMLTVTime convertit une date UTC ISO 8601 provenant de SQLite
// vers le format XMLTV.
//
// Exemple :
//
//	2026-08-24T15:58:00Z
//	→ 20260824155800 +0000
//
// Les dates du projet sont conservées en UTC.
func formatXMLTVTime(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(fmt.Sprintf(
			"date UTC invalide pour XMLTV: %q: %v",
			value,
			err,
		))
	}

	return t.UTC().Format("20060102150405 -0700")
}

// addSeconds ajoute un nombre de secondes à une date ISO 8601 UTC
// et retourne une date ISO 8601 UTC compatible avec SQLite.
func addSeconds(value string, seconds int) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(fmt.Sprintf(
			"date UTC invalide pour XMLTV: %q: %v",
			value,
			err,
		))
	}

	return t.UTC().
		Add(time.Duration(seconds) * time.Second).
		Format(time.RFC3339)
}
