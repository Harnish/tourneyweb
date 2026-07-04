package webhandler

import (
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

// monthGrid lays out a full calendar month as weeks of 7 days, padded with
// leading/trailing days from adjacent months so every week is complete.
func monthGrid(year int, month time.Month, gamesByDay map[string][]mydb.Game) [][]calendarDay {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	last := first.AddDate(0, 1, -1)
	gridStart := first.AddDate(0, 0, -int(first.Weekday()))
	gridEnd := last.AddDate(0, 0, 6-int(last.Weekday()))

	var weeks [][]calendarDay
	for d := gridStart; !d.After(gridEnd); d = d.AddDate(0, 0, 7) {
		var week []calendarDay
		for i := 0; i < 7; i++ {
			day := d.AddDate(0, 0, i)
			week = append(week, calendarDay{
				Date:    day,
				InMonth: day.Month() == month,
				Games:   gamesByDay[day.Format("2006-01-02")],
			})
		}
		weeks = append(weeks, week)
	}
	return weeks
}

func (me *Env) Calendar(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}

	year, month := t.StartDate.Year(), t.StartDate.Month()
	if v := r.URL.Query().Get("month"); v != "" {
		if parsed, err := time.Parse("2006-01", v); err == nil {
			year, month = parsed.Year(), parsed.Month()
		}
	}

	games := me.DB.AllGames(t.ID)
	gamesByDay := make(map[string][]mydb.Game)
	for _, g := range games {
		if g.Start.IsZero() {
			continue
		}
		key := g.Start.Format("2006-01-02")
		gamesByDay[key] = append(gamesByDay[key], g)
	}

	current := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	prev := current.AddDate(0, -1, 0)
	next := current.AddDate(0, 1, 0)

	me.render(w, "calendar", calendarData{
		baseData:   newBaseWithTournament(r, t),
		MonthLabel: current.Format("January 2006"),
		Weeks:      monthGrid(year, month, gamesByDay),
		PrevMonth:  prev.Format("2006-01"),
		NextMonth:  next.Format("2006-01"),
		Today:      time.Now().Format("2006-01-02"),
	})
}
