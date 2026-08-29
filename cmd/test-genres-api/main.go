package main

import (
	"fmt"
	"log"
	"sort"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/vmedia"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	client := vmedia.NewAPIClient(cfg)

	catalog, err := client.GetTVGenres()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Fournisseur : %s\n", cfg.Provider)
	fmt.Printf("Site ID     : %d\n", cfg.SiteID)
	fmt.Printf("AppName     : %s\n", cfg.AppName)
	fmt.Printf("Langue      : %s\n", cfg.Language)
	fmt.Printf("\nGenres TV retournés : %d\n\n", len(catalog.Genres))

	ids := make([]int, 0, len(catalog.Genres))

	for _, genre := range catalog.Genres {
		ids = append(ids, genre.ID)
	}

	sort.Ints(ids)

	fmt.Println("ID    TYPE    VODTYPE    NOM")
	fmt.Println("-----------------------------------------------")

	for _, id := range ids {
		genre := catalog.ByID[id]

		fmt.Printf(
			"%-5d %-7d %-10d %s\n",
			genre.ID,
			genre.GenreType,
			genre.VODType,
			genre.Name,
		)
	}
}
