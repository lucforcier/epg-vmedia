package main

import (
	"fmt"
	"log"
	"time"

	"epg-vmedia/internal/config"
	"epg-vmedia/internal/vmedia"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	client := vmedia.NewAPIClient(cfg)

	start := time.Date(2026, 8, 23, 3, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)

	channelFilter := []int{
		303125,
		303126,
		303127,
		303128,
	}

	items, total, pages, err := client.GetGuide(
		start,
		end,
		1,
		100,
		channelFilter,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("========================================")
	fmt.Println("TEST GUIDE AVEC channelFilter")
	fmt.Println("========================================")
	fmt.Println("Début       :", start)
	fmt.Println("Fin         :", end)
	fmt.Println("Chaînes     :", channelFilter)
	fmt.Println()
	fmt.Println("Programmes  :", len(items))
	fmt.Println("Total API   :", total)
	fmt.Println("Pages API   :", pages)
	fmt.Println()

	seenChannels := make(map[int]bool)

	for _, item := range items {
		seenChannels[item.RecentAirTime.ChannelID] = true
	}

	fmt.Println("Chaînes retournées :")
	for channelID := range seenChannels {
		fmt.Println("  ", channelID)
	}

	fmt.Println()
	fmt.Println("Programmes :")
	fmt.Println()

	for i, item := range items {
		fmt.Printf(
			"%3d | chaîne=%d | %s | %s\n",
			i+1,
			item.RecentAirTime.ChannelID,
			item.StartTime,
			item.Name,
		)
	}
}
