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

	channels, err := client.GetChannels()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Fournisseur : %s\n", cfg.Provider)
	fmt.Printf("Site ID     : %d\n", cfg.SiteID)
	fmt.Printf("AppName     : %s\n", cfg.AppName)
	fmt.Printf("Langue      : %s\n", cfg.Language)
	fmt.Printf("\nChaînes retournées : %d\n\n", len(channels))

	sort.Slice(channels, func(i, j int) bool {
		return channels[i].OrderNum < channels[j].OrderNum
	})

	fmt.Println("LCN   ID       CALLSIGN    NOM                         GENRES")
	fmt.Println("--------------------------------------------------------------------------")

	for _, channel := range channels {
		genres := ""

		for i, genreID := range channel.ContentGenres {
			if i > 0 {
				genres += ","
			}
			genres += fmt.Sprintf("%d", genreID)
		}

		fmt.Printf(
			"%-5d %-8d %-11s %-28s %s\n",
			channel.OrderNum,
			channel.ID,
			channel.CallSign,
			channel.Name,
			genres,
		)
	}
}
