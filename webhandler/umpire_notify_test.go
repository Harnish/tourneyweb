package webhandler

import (
	"sync"
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

func TestNotifyUmpireNilEmail(t *testing.T) {
	env := &Env{}
	env.notifyUmpire("a@x.com", "s", "m") // must not panic
	notifyWG.Wait()
}

func TestNotifyUmpireUsesSeam(t *testing.T) {
	var mu sync.Mutex
	var got []string
	old := sendNotice
	sendNotice = func(me *Env, to, subject, msg, link string) error {
		mu.Lock()
		got = append(got, to)
		mu.Unlock()
		return nil
	}
	defer func() { sendNotice = old }()
	env := &Env{}
	env.notifyUmpire("a@x.com", "s", "m")
	env.notifyUmpire("", "s", "m") // blank recipient skipped
	notifyWG.Wait()
	if len(got) != 1 || got[0] != "a@x.com" {
		t.Fatalf("got %v", got)
	}
}
