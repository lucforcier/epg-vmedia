package images

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Cache struct {
	BaseDir    string
	BaseURL    string
	HTTPClient *http.Client

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewCache(baseDir string, baseURL string) *Cache {
	return &Cache{
		BaseDir: baseDir,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		locks: make(map[string]*sync.Mutex),
	}
}

func (c *Cache) imagePath(imageID int, imageType int) string {
	return filepath.Join(
		c.BaseDir,
		fmt.Sprintf("%d-%d.jpg", imageID, imageType),
	)
}

func (c *Cache) imageLock(key string) *sync.Mutex {
	c.mu.Lock()
	defer c.mu.Unlock()

	if lock, ok := c.locks[key]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	c.locks[key] = lock

	return lock
}

func (c *Cache) Get(imageID int, imageType int) (string, error) {
	if imageID <= 0 {
		return "", fmt.Errorf("ID d'image invalide : %d", imageID)
	}

	if imageType <= 0 {
		return "", fmt.Errorf("type d'image invalide : %d", imageType)
	}

	filename := c.imagePath(imageID, imageType)

	lock := c.imageLock(filename)
	lock.Lock()
	defer lock.Unlock()

	// ------------------------------------------------------------
	// Image déjà présente
	// ------------------------------------------------------------

	if info, err := os.Stat(filename); err == nil {
		if info.Size() <= 0 {
			if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf(
					"suppression du fichier vide %s : %w",
					filename,
					err,
				)
			}
		} else {
			now := time.Now()

			if err := os.Chtimes(filename, now, now); err != nil {
				return "", fmt.Errorf(
					"mise à jour du temps d'accès %s : %w",
					filename,
					err,
				)
			}

			return filename, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf(
			"vérification de %s : %w",
			filename,
			err,
		)
	}

	// ------------------------------------------------------------
	// Cache miss : téléchargement VMedia
	// ------------------------------------------------------------

	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return "", fmt.Errorf(
			"création du répertoire %s : %w",
			c.BaseDir,
			err,
		)
	}

	url := fmt.Sprintf(
		"%s?id=%d&t=%d",
		c.BaseURL,
		imageID,
		imageType,
	)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return "", fmt.Errorf(
			"téléchargement %s : %w",
			url,
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf(
			"VMedia a retourné HTTP %d pour %s",
			resp.StatusCode,
			url,
		)
	}

	tmp := filename + ".tmp"

	file, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf(
			"création de %s : %w",
			tmp,
			err,
		)
	}

	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()

	if copyErr != nil {
		os.Remove(tmp)

		return "", fmt.Errorf(
			"écriture de %s : %w",
			tmp,
			copyErr,
		)
	}

	if closeErr != nil {
		os.Remove(tmp)

		return "", fmt.Errorf(
			"fermeture de %s : %w",
			tmp,
			closeErr,
		)
	}

	if err := os.Rename(tmp, filename); err != nil {
		os.Remove(tmp)

		return "", fmt.Errorf(
			"installation de %s : %w",
			filename,
			err,
		)
	}

	now := time.Now()

	if err := os.Chtimes(filename, now, now); err != nil {
		return "", fmt.Errorf(
			"mise à jour du temps d'utilisation %s : %w",
			filename,
			err,
		)
	}

	return filename, nil
}

func (c *Cache) Cleanup(maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, fmt.Errorf(
			"durée de conservation invalide : %s",
			maxAge,
		)
	}

	entries, err := os.ReadDir(c.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, fmt.Errorf(
			"lecture du cache %s : %w",
			c.BaseDir,
			err,
		)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Nous ne touchons qu'aux fichiers JPEG produits
		// par le cache.
		if filepath.Ext(name) != ".jpg" {
			continue
		}

		filename := filepath.Join(c.BaseDir, name)

		// Utiliser exactement le même verrou que Get().
		lock := c.imageLock(filename)
		lock.Lock()

		info, err := os.Stat(filename)
		if err != nil {
			lock.Unlock()

			if os.IsNotExist(err) {
				continue
			}

			return removed, fmt.Errorf(
				"vérification de %s pendant le nettoyage : %w",
				filename,
				err,
			)
		}

		// Le fichier peut avoir été utilisé entre ReadDir()
		// et l'acquisition du verrou. On consulte donc son mtime
		// après avoir obtenu le verrou.
		if !info.ModTime().Before(cutoff) {
			lock.Unlock()
			continue
		}

		err = os.Remove(filename)
		lock.Unlock()

		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return removed, fmt.Errorf(
				"suppression de %s : %w",
				filename,
				err,
			)
		}

		removed++
	}

	return removed, nil
}
