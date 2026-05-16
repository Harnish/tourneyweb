package mydb

import (
	"database/sql"
	"log/slog"
	"time"
)

type NewsItem struct {
	ID           int
	TournamentID int // 0 = site news
	Title        string
	Body         string
	CreatedAt    time.Time
	AuthorID     int
	AuthorName   string // joined from users
}

const newsSelect = `
	SELECT n.id, COALESCE(n.tournament_id, 0), n.title, n.body, n.created_at,
	       COALESCE(n.author_id, 0), COALESCE(u.name, '')
	FROM event_news n
	LEFT JOIN users u ON u.id = n.author_id`

func (me *MyDB) scanNews(rows *sql.Rows) []NewsItem {
	defer rows.Close()
	var out []NewsItem
	for rows.Next() {
		var n NewsItem
		if err := rows.Scan(&n.ID, &n.TournamentID, &n.Title, &n.Body, &n.CreatedAt, &n.AuthorID, &n.AuthorName); err != nil {
			slog.Error("scanNews", "err", err)
			continue
		}
		out = append(out, n)
	}
	return out
}

func (me *MyDB) AddNews(tournamentID int, title, body string, authorID int) int {
	var tid interface{}
	if tournamentID != 0 {
		tid = tournamentID
	}
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO event_news (tournament_id, title, body, author_id) VALUES ($1,$2,$3,$4) RETURNING id`,
		tid, title, body, authorID,
	).Scan(&id)
	if err != nil {
		slog.Error("AddNews", "err", err)
	}
	return id
}

func (me *MyDB) GetSiteNews() []NewsItem {
	rows, err := me.DB.Query(newsSelect + ` WHERE n.tournament_id IS NULL ORDER BY n.created_at DESC LIMIT 10`)
	if err != nil {
		slog.Error("GetSiteNews", "err", err)
		return nil
	}
	return me.scanNews(rows)
}

func (me *MyDB) GetTournamentNews(tournamentID int) []NewsItem {
	rows, err := me.DB.Query(newsSelect+` WHERE n.tournament_id=$1 ORDER BY n.created_at DESC`, tournamentID)
	if err != nil {
		slog.Error("GetTournamentNews", "err", err)
		return nil
	}
	return me.scanNews(rows)
}

func (me *MyDB) GetNewsByID(id int) (NewsItem, bool) {
	rows, err := me.DB.Query(newsSelect+` WHERE n.id=$1`, id)
	if err != nil {
		slog.Error("GetNewsByID", "err", err)
		return NewsItem{}, false
	}
	items := me.scanNews(rows)
	if len(items) == 0 {
		return NewsItem{}, false
	}
	return items[0], true
}

func (me *MyDB) UpdateNews(id int, title, body string) {
	_, err := me.DB.Exec(`UPDATE event_news SET title=$1, body=$2 WHERE id=$3`, title, body, id)
	if err != nil {
		slog.Error("UpdateNews", "err", err)
	}
}

func (me *MyDB) DeleteNews(id int) {
	me.DB.Exec(`DELETE FROM event_news WHERE id=$1`, id)
}
