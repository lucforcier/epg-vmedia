package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Job est une tâche exécutée par le scheduler.
type Job func() error

// Scheduler exécute une tâche quotidiennement à une heure donnée.
type Scheduler struct {
	enabled  bool
	timeText string
	location *time.Location
	job      Job
}

// WeeklyScheduler exécute une tâche une fois par semaine,
// à un jour et une heure donnés.
type WeeklyScheduler struct {
	enabled  bool
	name     string
	day      time.Weekday
	timeText string
	location *time.Location
	job      Job
}

// New crée un scheduler quotidien.
func New(
	enabled bool,
	timeText string,
	locationName string,
	job Job,
) (*Scheduler, error) {

	location, err := time.LoadLocation(locationName)
	if err != nil {
		return nil, err
	}

	if _, err := time.Parse("15:04", timeText); err != nil {
		return nil, err
	}

	if job == nil {
		return nil, fmt.Errorf("job scheduler nil")
	}

	return &Scheduler{
		enabled:  enabled,
		timeText: timeText,
		location: location,
		job:      job,
	}, nil
}

// Start démarre le scheduler quotidien dans une goroutine.
//
// La fonction retourne immédiatement.
//
// La première exécution aura lieu à la prochaine occurrence
// de l'heure configurée.
func (s *Scheduler) Start(ctx context.Context) {

	if !s.enabled {
		log.Println("Scheduler EPG désactivé")
		return
	}

	go s.run(ctx)
}

func (s *Scheduler) run(ctx context.Context) {

	log.Printf(
		"Scheduler EPG activé : tous les jours à %s (%s)",
		s.timeText,
		s.location.String(),
	)

	for {
		now := time.Now().In(s.location)
		next := s.nextRun(now)

		wait := time.Until(next)

		log.Printf(
			"Prochaine génération XMLTV : %s",
			next.Format(time.RFC3339),
		)

		timer := time.NewTimer(wait)

		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			log.Println("Scheduler EPG arrêté")
			return

		case <-timer.C:
		}

		log.Println("========================================")
		log.Println("GÉNÉRATION XMLTV PLANIFIÉE")
		log.Println("========================================")

		if err := s.job(); err != nil {
			log.Printf(
				"Erreur génération XMLTV planifiée : %v",
				err,
			)
		} else {
			log.Println(
				"Génération XMLTV planifiée terminée avec succès",
			)
		}

		// La prochaine occurrence est recalculée après le travail.
	}
}

// nextRun retourne la prochaine occurrence quotidienne.
func (s *Scheduler) nextRun(now time.Time) time.Time {

	parsed, _ := time.Parse("15:04", s.timeText)

	next := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		parsed.Hour(),
		parsed.Minute(),
		0,
		0,
		s.location,
	)

	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}

	return next
}

// NewWeekly crée un scheduler hebdomadaire.
//
// name est un nom descriptif utilisé uniquement dans les journaux.
//
// day utilise la convention ISO :
//
//	1 = lundi
//	2 = mardi
//	...
//	7 = dimanche
func NewWeekly(
	enabled bool,
	name string,
	day int,
	timeText string,
	locationName string,
	job Job,
) (*WeeklyScheduler, error) {

	location, err := time.LoadLocation(locationName)
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf(
			"nom du scheduler hebdomadaire vide",
		)
	}

	if day < 1 || day > 7 {
		return nil, fmt.Errorf(
			"jour hebdomadaire invalide : %d",
			day,
		)
	}

	if _, err := time.Parse("15:04", timeText); err != nil {
		return nil, err
	}

	if job == nil {
		return nil, fmt.Errorf(
			"job scheduler hebdomadaire nil",
		)
	}

	return &WeeklyScheduler{
		enabled:  enabled,
		name:     name,
		day:      isoWeekday(day),
		timeText: timeText,
		location: location,
		job:      job,
	}, nil
}

// Start démarre le scheduler hebdomadaire dans une goroutine.
func (s *WeeklyScheduler) Start(ctx context.Context) {

	if !s.enabled {
		log.Printf(
			"Scheduler hebdomadaire désactivé : %s",
			s.name,
		)
		return
	}

	go s.run(ctx)
}

func (s *WeeklyScheduler) run(ctx context.Context) {

	log.Printf(
		"Scheduler hebdomadaire activé : %s — chaque %s à %s (%s)",
		s.name,
		weekdayName(s.day),
		s.timeText,
		s.location.String(),
	)

	for {
		now := time.Now().In(s.location)
		next := s.nextRun(now)

		wait := time.Until(next)

		log.Printf(
			"Prochaine exécution %s : %s",
			s.name,
			next.Format(time.RFC3339),
		)

		timer := time.NewTimer(wait)

		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			log.Printf(
				"Scheduler hebdomadaire arrêté : %s",
				s.name,
			)
			return

		case <-timer.C:
		}

		log.Println("========================================")
		log.Printf(
			"TÂCHE HEBDOMADAIRE PLANIFIÉE : %s",
			s.name,
		)
		log.Println("========================================")

		if err := s.job(); err != nil {
			log.Printf(
				"Erreur tâche hebdomadaire %s : %v",
				s.name,
				err,
			)
		} else {
			log.Printf(
				"Tâche hebdomadaire %s terminée avec succès",
				s.name,
			)
		}

		// La prochaine occurrence est recalculée après le travail.
	}
}

// nextRun retourne la prochaine occurrence hebdomadaire.
func (s *WeeklyScheduler) nextRun(now time.Time) time.Time {

	parsed, _ := time.Parse("15:04", s.timeText)

	next := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		parsed.Hour(),
		parsed.Minute(),
		0,
		0,
		s.location,
	)

	daysUntil := (int(s.day) - int(next.Weekday()) + 7) % 7

	next = next.AddDate(0, 0, daysUntil)

	if !next.After(now) {
		next = next.AddDate(0, 0, 7)
	}

	return next
}

// isoWeekday convertit 1=lundi ... 7=dimanche vers time.Weekday.
func isoWeekday(day int) time.Weekday {

	if day == 7 {
		return time.Sunday
	}

	return time.Weekday(day)
}

func weekdayName(day time.Weekday) string {

	switch day {
	case time.Monday:
		return "lundi"
	case time.Tuesday:
		return "mardi"
	case time.Wednesday:
		return "mercredi"
	case time.Thursday:
		return "jeudi"
	case time.Friday:
		return "vendredi"
	case time.Saturday:
		return "samedi"
	case time.Sunday:
		return "dimanche"
	default:
		return "jour inconnu"
	}
}
