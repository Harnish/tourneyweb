package webhandler

import (
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func TestGameLine(t *testing.T) {
	g := mydb.Game{
		Division: mydb.Division{Name: "10U"},
		HomeTeam: mydb.Team{Name: "Dawg Pound"},
		AwayTeam: mydb.Team{Name: "River Rats"},
		Location: "Field 1",
		Start:    time.Date(2026, 7, 10, 9, 0, 0, 0, time.Local),
	}
	want := "10U · Dawg Pound vs River Rats · Jul 10, 2026 9:00 AM · Field 1"
	if got := gameLine(g); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	g.Start = time.Time{}
	want = "10U · Dawg Pound vs River Rats · Field 1"
	if got := gameLine(g); got != want {
		t.Fatalf("zero start: got %q want %q", got, want)
	}
}
