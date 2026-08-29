package vmedia

import (
	"encoding/json"
	"fmt"
	"os"
)

type Channel struct {
	OrderNum              int     `json:"orderNum"`
	Visible               bool    `json:"visible"`
	OriginalLiveUTCOffset int     `json:"originalLiveUtcOffset"`
	ARX                   int     `json:"arX"`
	ARY                   int     `json:"arY"`
	AccessNum             int     `json:"accessNum"`
	UTCOffset             int     `json:"UtcOffset"`
	IsBDU                 bool    `json:"IsBdu"`
	IsDescriptiveAudio    bool    `json:"IsDescriptiveAudio"`
	CallSign              string  `json:"CallSign"`
	IsRestartAvailable    bool    `json:"IsRestartAvailable"`
	DVRWindow             int     `json:"DvrWindow"`
	IsVCloud              bool    `json:"isVcloud"`
	ProviderChannelID     string  `json:"ProviderChannelId"`
	HasRecordings         bool    `json:"HasRecordings"`
	HasSubtitles          bool    `json:"HasSubtitles"`
	Name                  string  `json:"name"`
	ContentGenres         []int   `json:"contentGenres"`
	EpisodeNum            int     `json:"episodeNum"`
	ContainerID           int     `json:"containerID"`
	Description           string  `json:"description"`
	ItemDate              string  `json:"item_date"`
	ImageIDs              []int   `json:"ImageIds"`
	Images                []Image `json:"Images"`
	ContentType           int     `json:"contentType"`
	ID                    int     `json:"id"`
}

type Image struct {
	ID   int `json:"Id"`
	Type int `json:"Type"`
}

type channelResponse struct {
	Items []Channel `json:"items"`
}

// LoadChannels lit le catalogue de chaînes VMedia à partir d'un fichier JSON.
func LoadChannels(filename string) ([]Channel, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("lecture de %s : %w", filename, err)
	}

	var response channelResponse

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("décodage de %s : %w", filename, err)
	}

	return response.Items, nil
}

// GetChannels récupère le catalogue des chaînes depuis l'API VMedia.
func (c *APIClient) GetChannels() ([]Channel, error) {
	request := struct {
		SiteID  int    `json:"siteId"`
		AppName string `json:"appName"`
		SLang   string `json:"sLang"`
		Request struct {
			Type   int `json:"type"`
			Paging struct {
				PageNumber  int `json:"pageNumber"`
				ItemsOnPage int `json:"itemsOnPage"`
			} `json:"paging"`
		} `json:"request"`
	}{
		SiteID:  c.SiteID,
		AppName: c.AppName,
		SLang:   c.Language,
	}

	request.Request.Type = 4
	request.Request.Paging.PageNumber = 1
	request.Request.Paging.ItemsOnPage = 500

	var response channelResponse

	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encodage de la requête: %w", err)
	}

	fmt.Printf("REQUÊTE CHANNEL : %s\n", data)

	if err := c.post("/api/channel/app", request, &response); err != nil {
		return nil, fmt.Errorf("récupération des chaînes: %w", err)
	}

	return response.Items, nil
}

func (c Channel) ColorLogoURL() string {
	for _, image := range c.Images {
		if image.Type == 10 && image.ID != 0 {
			return fmt.Sprintf(
				"https://images.vmedia.ca/api/images?id=%d&t=10",
				image.ID,
			)
		}
	}

	return ""
}

func (c Channel) MonochromeLogoURL() string {
	for _, image := range c.Images {
		if image.Type == 8 && image.ID != 0 {
			return fmt.Sprintf(
				"https://images.vmedia.ca/api/images?id=%d&t=8",
				image.ID,
			)
		}
	}

	return ""
}
