package vmedia

import (
	"fmt"
	"time"
)

type GuideImage struct {
	ID   int `json:"Id"`
	Type int `json:"Type"`
}

type GuideItem struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	ShortDescription  string `json:"shortDescription"`
	StartTime         string `json:"startTime"`
	Length            int    `json:"length"`
	ProviderProgramID string `json:"providerProgramId"`

	ProgramID   int `json:"ProgramId"`
	ContainerID int `json:"containerID"`
	ImageCount  int `json:"imageCount"`

	ImageIDs []int `json:"ImageIds"`

	Images       []GuideImage `json:"Images"`
	SeriesImages []GuideImage `json:"SeriesImages"`

	RecentAirTime struct {
		ChannelID int `json:"channelID"`

		Channel struct {
			Images []Image `json:"Images"`
		} `json:"Channel"`
	} `json:"recentAirTime"`
}

type guideResponse struct {
	Items  []GuideItem `json:"items"`
	Paging struct {
		TotalItems int `json:"totalItems"`
		TotalPages int `json:"totalPages"`
	} `json:"paging"`
}

// GetGuide récupère une page du guide TV VMedia pour les chaînes sélectionnées.
func (c *APIClient) GetGuide(
	start time.Time,
	end time.Time,
	page int,
	itemsOnPage int,
	channelFilter []int,
) ([]GuideItem, int, int, error) {

	request := struct {
		SiteID  int    `json:"siteId"`
		AppName string `json:"appName"`
		SLang   string `json:"sLang"`
		Request struct {
			Type          int   `json:"type"`
			ChannelFilter []int `json:"channelFilter"`
			Paging        struct {
				PageNumber  int `json:"pageNumber"`
				ItemsOnPage int `json:"itemsOnPage"`
			} `json:"paging"`
			Filter struct {
				DontFetchDetails bool   `json:"DontFetchDetails"`
				Date             string `json:"date"`
				DateTill         string `json:"dateTill"`
			} `json:"filter"`
		} `json:"request"`
	}{
		SiteID:  c.SiteID,
		AppName: c.AppName,
		SLang:   c.Language,
	}

	request.Request.Type = 4
	request.Request.ChannelFilter = channelFilter
	request.Request.Paging.PageNumber = page
	request.Request.Paging.ItemsOnPage = itemsOnPage
	request.Request.Filter.DontFetchDetails = false
	request.Request.Filter.Date = start.UTC().Format("2006-01-02T15:04:05.000Z")
	request.Request.Filter.DateTill = end.UTC().Format("2006-01-02T15:04:05.000Z")

	var response guideResponse

	if err := c.post("/api/guide/app", request, &response); err != nil {
		return nil, 0, 0, fmt.Errorf("récupération du guide: %w", err)
	}

	return response.Items, response.Paging.TotalItems, response.Paging.TotalPages, nil
}

// ParseVMediaDate convertit un timestamp VMedia de la forme
// /Date(1787...000)/ en time.Time UTC.
func ParseVMediaDate(value string) (time.Time, error) {
	var milliseconds int64

	if _, err := fmt.Sscanf(value, "/Date(%d)/", &milliseconds); err != nil {
		return time.Time{}, fmt.Errorf(
			"timestamp VMedia invalide %q: %w",
			value,
			err,
		)
	}

	return time.UnixMilli(milliseconds).UTC(), nil
}
