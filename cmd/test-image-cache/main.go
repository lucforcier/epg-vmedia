package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"epg-vmedia/internal/images"
)

func main() {
	const (
		imageID   = 32736662
		imageType = 28
		cacheDir  = "/data/image-cache-test"
	)

	// ------------------------------------------------------------
	// Nettoyage du test précédent
	// ------------------------------------------------------------

	if err := os.RemoveAll(cacheDir); err != nil {
		log.Fatalf("Nettoyage du cache de test : %v", err)
	}

	cache := images.NewCache(
		cacheDir,
		"https://images.vmedia.ca/api/images",
	)

	fmt.Println("========================================")
	fmt.Println("TEST CACHE IMAGE")
	fmt.Println("========================================")
	fmt.Printf("Image VMedia : %d\n", imageID)
	fmt.Printf("Type         : %d\n", imageType)
	fmt.Printf("Cache        : %s\n", cacheDir)
	fmt.Println()

	// ------------------------------------------------------------
	// Premier appel : cache miss
	// ------------------------------------------------------------

	fmt.Println("1. Premier appel — téléchargement attendu")

	path, err := cache.Get(imageID, imageType)
	if err != nil {
		log.Fatalf("Premier appel : %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("Stat après téléchargement : %v", err)
	}

	fmt.Printf("   Fichier : %s\n", path)
	fmt.Printf("   Taille  : %d octets\n", info.Size())
	fmt.Printf("   mtime   : %s\n", info.ModTime().Format(time.RFC3339Nano))

	firstModTime := info.ModTime()

	fmt.Println()

	// ------------------------------------------------------------
	// Petite pause pour rendre le changement de mtime visible
	// ------------------------------------------------------------

	time.Sleep(2 * time.Second)

	// ------------------------------------------------------------
	// Deuxième appel : cache hit
	// ------------------------------------------------------------

	fmt.Println("2. Deuxième appel — cache attendu")

	path2, err := cache.Get(imageID, imageType)
	if err != nil {
		log.Fatalf("Deuxième appel : %v", err)
	}

	info2, err := os.Stat(path2)
	if err != nil {
		log.Fatalf("Stat après cache hit : %v", err)
	}

	fmt.Printf("   Fichier : %s\n", path2)
	fmt.Printf("   Taille  : %d octets\n", info2.Size())
	fmt.Printf("   mtime   : %s\n", info2.ModTime().Format(time.RFC3339Nano))

	fmt.Println()

	// ------------------------------------------------------------
	// Vérifications
	// ------------------------------------------------------------

	if filepath.Clean(path) != filepath.Clean(path2) {
		log.Fatalf("Les chemins diffèrent : %s / %s", path, path2)
	}

	if info2.ModTime().After(firstModTime) {
		fmt.Println("OK : le mtime a été actualisé lors du cache hit.")
	} else {
		log.Fatalf("ERREUR : le mtime n'a pas été actualisé.")
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("TEST RÉUSSI")
	fmt.Println("========================================")
}
