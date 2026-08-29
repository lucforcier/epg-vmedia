package xmltv

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// XMLTV est la racine du document XMLTV.
type XMLTV struct {
	XMLName   xml.Name   `xml:"tv"`
	Generator string     `xml:"generator-info-name,attr,omitempty"`
	Channels  []XMLChannel `xml:"channel"`
	Programs  []XMLProgram `xml:"programme"`
}

// XMLChannel représente un élément <channel>.
type XMLChannel struct {
	ID          string      `xml:"id,attr"`
	DisplayName XMLText     `xml:"display-name"`
	Icon        *XMLIcon    `xml:"icon,omitempty"`
}

// XMLProgram représente un élément <programme>.
type XMLProgram struct {
	Start   string `xml:"start,attr"`
	Stop    string `xml:"stop,attr"`
	Channel string `xml:"channel,attr"`

	Title       XMLText    `xml:"title"`
	Desc        *XMLText   `xml:"desc,omitempty"`
	Icon        *XMLIcon   `xml:"icon,omitempty"`
}

// XMLText représente un texte XMLTV avec langue française.
type XMLText struct {
	Lang  string `xml:"lang,attr,omitempty"`
	Value string `xml:",chardata"`
}

// XMLIcon représente <icon src="..."/>.
type XMLIcon struct {
	Src string `xml:"src,attr"`
}

// Write écrit un document XMLTV complet dans w.
//
// Les données doivent déjà avoir été filtrées par la couche SQLite.
// Cette fonction ne fait aucun accès réseau.
func Write(
	w io.Writer,
	channels []Channel,
	events []Event,
	baseURL string,
	logo string,
) error {

	baseURL = strings.TrimRight(baseURL, "/")

	tv := XMLTV{
		Generator: "epg-vmedia",
		Channels:  make([]XMLChannel, 0, len(channels)),
		Programs:  make([]XMLProgram, 0, len(events)),
	}

	for _, channel := range channels {
		id := fmt.Sprintf("%d", channel.ID)

		preference := logo
		if preference == "" {
			preference = channel.LogoPreference
		}

		if preference != "color" && preference != "mono" {
			return fmt.Errorf(
				"préférence de logo invalide pour la chaîne %d: %q",
				channel.ID,
				preference,
			)
		}

		xmlChannel := XMLChannel{
			ID: id,
			DisplayName: XMLText{
				Lang:  "fr",
				Value: channel.Name,
			},
		}

		xmlChannel.Icon = &XMLIcon{
			Src: fmt.Sprintf(
				"%s/logos/%d/%s.png",
				baseURL,
				channel.ID,
				preference,
			),
		}

		tv.Channels = append(tv.Channels, xmlChannel)
	}

	for _, event := range events {
		program := XMLProgram{
			Start:   formatXMLTVTime(event.StartTimeUTC),
			Stop:    formatXMLTVTime(addSeconds(event.StartTimeUTC, event.DurationSeconds)),
			Channel: fmt.Sprintf("%d", event.ChannelID),

			Title: XMLText{
				Lang:  "fr",
				Value: event.Title,
			},
		}

		description := strings.TrimSpace(event.Description)
		if description == "" {
			description = strings.TrimSpace(event.ShortDescription)
		}

		if description != "" {
			program.Desc = &XMLText{
				Lang:  "fr",
				Value: description,
			}
		}

		if event.HasProgramImage {
			program.Icon = &XMLIcon{
				Src: fmt.Sprintf(
					"%s/images/%d-%d.jpg",
					baseURL,
					event.ProgramImageID,
					event.ProgramImageType,
				),
			}
		}

		tv.Programs = append(tv.Programs, program)
	}

	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return fmt.Errorf("écriture en-tête XMLTV: %w", err)
	}

	if err := encoder.Encode(tv); err != nil {
		return fmt.Errorf("encodage XMLTV: %w", err)
	}

	if err := encoder.Flush(); err != nil {
		return fmt.Errorf("finalisation XMLTV: %w", err)
	}

	return nil
}

