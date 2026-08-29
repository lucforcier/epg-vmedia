package vmedia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"epg-vmedia/internal/config"
)

// APIClient représente un client de l'API VMedia.
type APIClient struct {
	BaseURL    string
	SiteID     int
	AppName    string
	Language   string
	HTTPClient *http.Client
}

// Paramètres de résilience réseau.
//
// Une requête peut donc durer jusqu'à 60 secondes avant d'être
// considérée comme échouée. Les erreurs récupérables sont ensuite
// retentées avec un délai croissant.
const (
	httpTimeout    = 60 * time.Second
	maxHTTPRetries = 5
)

// NewAPIClient crée un client API à partir de la configuration générale.
func NewAPIClient(cfg config.Config) *APIClient {
	return &APIClient{
		BaseURL:  strings.TrimRight(cfg.APIURL, "/"),
		SiteID:   cfg.SiteID,
		AppName:  cfg.AppName,
		Language: cfg.Language,
		HTTPClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// post effectue une requête POST JSON vers l'API VMedia.
//
// Les erreurs réseau et les réponses HTTP 5xx sont retentées.
// Les erreurs HTTP 4xx et les erreurs de décodage JSON ne sont pas
// retentées puisqu'elles indiquent généralement un problème permanent
// avec la requête ou les données retournées.
func (c *APIClient) post(
	path string,
	requestBody any,
	responseBody any,
) error {

	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encodage JSON: %w", err)
	}

	url := c.BaseURL + path

	var lastErr error

	for attempt := 1; attempt <= maxHTTPRetries; attempt++ {

		req, err := http.NewRequest(
			http.MethodPost,
			url,
			bytes.NewReader(body),
		)
		if err != nil {
			return fmt.Errorf(
				"création requête %s: %w",
				url,
				err,
			)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.HTTPClient.Do(req)

		// ------------------------------------------------------------
		// Erreur réseau / timeout
		// ------------------------------------------------------------

		if err != nil {

			lastErr = fmt.Errorf(
				"requête %s: %w",
				url,
				err,
			)

			if attempt < maxHTTPRetries {

				delay := retryDelay(attempt)

				fmt.Printf(
					"  API VMedia : tentative %d/%d échouée (%v), nouvelle tentative dans %s\n",
					attempt,
					maxHTTPRetries,
					err,
					delay,
				)

				time.Sleep(delay)
				continue
			}

			break
		}

		// ------------------------------------------------------------
		// Lecture de la réponse
		// ------------------------------------------------------------

		responseData, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {

			lastErr = fmt.Errorf(
				"lecture réponse %s: %w",
				url,
				readErr,
			)

			if attempt < maxHTTPRetries {

				delay := retryDelay(attempt)

				fmt.Printf(
					"  API VMedia : lecture réponse échouée, tentative %d/%d, nouvelle tentative dans %s\n",
					attempt,
					maxHTTPRetries,
					delay,
				)

				time.Sleep(delay)
				continue
			}

			break
		}

		// ------------------------------------------------------------
		// HTTP 2xx
		// ------------------------------------------------------------

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {

			if err := json.Unmarshal(responseData, responseBody); err != nil {
				return fmt.Errorf(
					"décodage réponse %s: %w",
					url,
					err,
				)
			}

			return nil
		}

		// ------------------------------------------------------------
		// HTTP 5xx : erreur serveur récupérable
		// ------------------------------------------------------------

		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {

			lastErr = fmt.Errorf(
				"API %s: HTTP %d: %s",
				url,
				resp.StatusCode,
				strings.TrimSpace(string(responseData)),
			)

			if attempt < maxHTTPRetries {

				delay := retryDelay(attempt)

				fmt.Printf(
					"  API VMedia : HTTP %d, tentative %d/%d, nouvelle tentative dans %s\n",
					resp.StatusCode,
					attempt,
					maxHTTPRetries,
					delay,
				)

				time.Sleep(delay)
				continue
			}

			break
		}

		// ------------------------------------------------------------
		// HTTP 4xx ou autre erreur permanente
		// ------------------------------------------------------------

		return fmt.Errorf(
			"API %s: HTTP %d: %s",
			url,
			resp.StatusCode,
			strings.TrimSpace(string(responseData)),
		)
	}

	return fmt.Errorf(
		"échec après %d tentatives: %w",
		maxHTTPRetries,
		lastErr,
	)
}

// retryDelay retourne le délai avant la tentative suivante.
//
// 2 s, 4 s, 8 s, 16 s.
func retryDelay(attempt int) time.Duration {
	return 2 * time.Second * time.Duration(1<<(attempt-1))
}
