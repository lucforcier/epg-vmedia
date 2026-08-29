package vmedia

import (
	"encoding/json"
	"fmt"
	"os"
)

// Genre représente un genre/catégorie provenant de l'API VMedia.
type Genre struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ChildrenIDs []int  `json:"childrenIDs"`
	ItemCount   int    `json:"itemCount"`
	GenreType   int    `json:"genreType"`
	ParentID    int    `json:"parentID"`
	Advisory    int    `json:"advisory"`
	VODType     int    `json:"vodType"`
	OrderNumber int    `json:"orderNumber"`
	Description string `json:"Description"`
	Type        int    `json:"Type"`
}

// GenresResponse représente la réponse complète du fichier genres.json.
type GenresResponse struct {
	Genres []Genre `json:"genres"`
}

// GenreCatalog permet de rechercher rapidement les genres par ID.
type GenreCatalog struct {
	Genres []Genre
	ByID   map[int]Genre
}

// LoadGenres charge le catalogue des genres VMedia.
func LoadGenres(filename string) (*GenreCatalog, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("lecture de %s : %w", filename, err)
	}

	var response GenresResponse

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("décodage de %s : %w", filename, err)
	}

	catalog := &GenreCatalog{
		Genres: response.Genres,
		ByID:   make(map[int]Genre, len(response.Genres)),
	}

	for _, genre := range response.Genres {
		catalog.ByID[genre.ID] = genre
	}

	return catalog, nil
}

// Get retourne un genre à partir de son ID.
func (c *GenreCatalog) Get(id int) (Genre, bool) {
	genre, ok := c.ByID[id]
	return genre, ok
}

// Names retourne les noms correspondant à une liste d'IDs de genres.
func (c *GenreCatalog) Names(ids []int) []string {
	names := make([]string, 0, len(ids))

	for _, id := range ids {
		if genre, ok := c.ByID[id]; ok {
			names = append(names, genre.Name)
		}
	}

	return names
}

// GetTVGenres récupère dynamiquement les genres TV auprès de l'API VMedia.
//
// Le type 1 correspond à ContentGenreType.TV_GENRE_TYPE.
// Les noms et les IDs ne sont volontairement pas codés en dur.
func (c *APIClient) GetTVGenres() (*GenreCatalog, error) {
	request := struct {
		SiteID   int    `json:"siteID"`
		AppName  string `json:"appName"`
		Language string `json:"sLang"`
		Type     int    `json:"type"`
		VODType  int    `json:"vodType"`
	}{
		SiteID:   c.SiteID,
		AppName:  c.AppName,
		Language: c.Language,
		Type:     1,
		VODType:  1,
	}

	var response GenresResponse

	if err := c.post("/api/genres/app", request, &response); err != nil {
		return nil, fmt.Errorf("récupération des genres TV: %w", err)
	}

	catalog := &GenreCatalog{
		Genres: response.Genres,
		ByID:   make(map[int]Genre, len(response.Genres)),
	}

	for _, genre := range response.Genres {
		catalog.ByID[genre.ID] = genre
	}

	return catalog, nil
}
