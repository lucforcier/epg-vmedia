package logos

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type Cache struct {
	BaseDir    string
	BaseURL    string
	HTTPClient *http.Client
}

func NewCache(baseDir string, baseURL string) *Cache {
	return &Cache{
		BaseDir:    baseDir,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	}
}

func (c *Cache) logoPath(channelID int, variant string) string {
	return filepath.Join(
		c.BaseDir,
		fmt.Sprintf("%d", channelID),
		variant+".png",
	)
}

func (c *Cache) download(
	channelID int,
	imageID int,
	imageType int,
	variant string,
	force bool,
) error {
	if imageID == 0 {
		return fmt.Errorf(
			"aucun ID d'image pour le logo %s",
			variant,
		)
	}

	if c.BaseURL == "" {
		return fmt.Errorf("URL de base des images non configurée")
	}

	filename := c.logoPath(channelID, variant)

	// En mode normal, conserver le fichier déjà en cache.
	// En mode forcé, toujours télécharger une nouvelle copie.
	if !force {
		if info, err := os.Stat(filename); err == nil {
			if info.Size() > 0 {
				fmt.Printf(
					"Déjà en cache : %s (%d octets)\n",
					filename,
					info.Size(),
				)
				return nil
			}

			fmt.Printf(
				"Fichier vide, téléchargement : %s\n",
				filename,
			)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf(
				"vérification de %s : %w",
				filename,
				err,
			)
		}
	}

	dir := filepath.Join(
		c.BaseDir,
		fmt.Sprintf("%d", channelID),
	)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf(
			"création du répertoire %s : %w",
			dir,
			err,
		)
	}

	url := fmt.Sprintf(
		"%s?id=%d&t=%d",
		c.BaseURL,
		imageID,
		imageType,
	)

	if force {
		fmt.Printf(
			"Rafraîchissement forcé : %s\n",
			url,
		)
	} else {
		fmt.Printf(
			"Téléchargement : %s\n",
			url,
		)
	}

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf(
			"téléchargement %s : %w",
			url,
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"VMedia a retourné HTTP %d pour %s",
			resp.StatusCode,
			url,
		)
	}

	tmp := filename + ".tmp"

	file, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf(
			"création de %s : %w",
			tmp,
			err,
		)
	}

	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()

	if copyErr != nil {
		os.Remove(tmp)
		return fmt.Errorf(
			"écriture de %s : %w",
			tmp,
			copyErr,
		)
	}

	if closeErr != nil {
		os.Remove(tmp)
		return fmt.Errorf(
			"fermeture de %s : %w",
			tmp,
			closeErr,
		)
	}

	if err := os.Rename(tmp, filename); err != nil {
		os.Remove(tmp)
		return fmt.Errorf(
			"installation de %s : %w",
			filename,
			err,
		)
	}

	fmt.Printf(
		"  -> %s\n",
		filename,
	)

	return nil
}

func (c *Cache) DownloadColor(channelID int, imageID int) error {
	return c.download(
		channelID,
		imageID,
		10,
		"color",
		false,
	)
}

func (c *Cache) DownloadMono(channelID int, imageID int) error {
	return c.download(
		channelID,
		imageID,
		8,
		"mono",
		false,
	)
}

func (c *Cache) RefreshColor(channelID int, imageID int) error {
	return c.download(
		channelID,
		imageID,
		10,
		"color",
		true,
	)
}

func (c *Cache) RefreshMono(channelID int, imageID int) error {
	return c.download(
		channelID,
		imageID,
		8,
		"mono",
		true,
	)
}
