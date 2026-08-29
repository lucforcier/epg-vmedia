package main

import (
	"fmt"
	"log"

	"epg-vmedia/internal/database"
)

func main() {
	db, err := database.Open("/data/epg.db")
	if err != nil {
		log.Fatalf("Erreur ouverture SQLite : %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT
			id,
			access_num,
			name,
			callsign,
			enabled,
			color_logo_id,
			mono_logo_id
		FROM channels
		ORDER BY access_num
		LIMIT 20
	`)
	if err != nil {
		log.Fatalf("Erreur requête chaînes : %v", err)
	}
	defer rows.Close()

	fmt.Println()
	fmt.Println("================================================================================================================")
	fmt.Printf("%-10s %-6s %-40s %-12s %-8s %-15s %-15s\n",
		"ID", "LCN", "NOM", "CALLSIGN", "ENABLED", "LOGO COULEUR", "LOGO MONO")
	fmt.Println("================================================================================================================")

	for rows.Next() {
		var (
			id          int
			accessNum   int
			name        string
			callsign    string
			enabled     int
			colorLogoID int
			monoLogoID  int
		)

		if err := rows.Scan(
			&id,
			&accessNum,
			&name,
			&callsign,
			&enabled,
			&colorLogoID,
			&monoLogoID,
		); err != nil {
			log.Fatalf("Erreur lecture chaîne : %v", err)
		}

		fmt.Printf("%-10d %-6d %-40.40s %-12.12s %-8d %-15d %-15d\n",
			id,
			accessNum,
			name,
			callsign,
			enabled,
			colorLogoID,
			monoLogoID,
		)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Erreur parcours chaînes : %v", err)
	}

	var (
		total      int
		selected   int
		colorLogos int
		monoLogos  int
	)

	err = db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(enabled), 0),
			COALESCE(SUM(CASE WHEN color_logo_id > 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN mono_logo_id > 0 THEN 1 ELSE 0 END), 0)
		FROM channels
	`).Scan(
		&total,
		&selected,
		&colorLogos,
		&monoLogos,
	)

	if err != nil {
		log.Fatalf("Erreur statistiques : %v", err)
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("STATISTIQUES SQLITE")
	fmt.Println("========================================")
	fmt.Printf("Chaînes totales       : %d\n", total)
	fmt.Printf("Chaînes sélectionnées : %d\n", selected)
	fmt.Printf("Logos couleur         : %d\n", colorLogos)
	fmt.Printf("Logos monochrome      : %d\n", monoLogos)
	fmt.Println("========================================")
}
