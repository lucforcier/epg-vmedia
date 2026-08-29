package xmltv

// Options contrôle la génération d'un guide XMLTV.
type Options struct {
	// Channels contient les IDs SQLite des chaînes à publier.
	// Une liste vide signifie : toutes les chaînes enabled.
	Channels []int

	// Logo est la préférence de logo à utiliser.
	// Valeurs acceptées : "color" ou "mono".
	Logo string

	// Days est le nombre de jours à publier.
	// La valeur doit être supérieure à zéro.
	Days int

	// BaseURL est l'URL publique utilisée pour les ressources locales.
	// Exemple : http://192.168.1.20:5580
	BaseURL string
}

// Channel représente une chaîne XMLTV à publier.
type Channel struct {
	ID               int
	AccessNum        int
	Name             string
	CallSign         string
	LogoPreference   string
	ColorLogoID      int64
	MonoLogoID       int64
}

// Event représente une occurrence destinée à XMLTV.
type Event struct {
	EventID          int64
	ChannelID        int
	ChannelName      string
	ChannelAccessNum int

	Title            string
	Description      string
	ShortDescription string

	StartTimeUTC    string
	DurationSeconds int

	ProgramImageID   int64
	ProgramImageType int
	HasProgramImage  bool
}
