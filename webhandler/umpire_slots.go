package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

type slotChoice struct {
	Slot   int
	UserID int
	Name   string
}

type umpireOption struct {
	UserID  int
	Label   string
	Blocked bool
}

type slotView struct {
	Slot       int
	UserID     int
	FreeName   string
	Status     string
	DeclinedBy string
}

// slotBlock is the data for the "umpireSlots" template used by every game form.
type slotBlock struct {
	Slots   []slotView
	Umpires []umpireOption
}

// slotCount is how many slots a game form shows: the division's umpire count, or more if the game already uses higher slots.
func (me *Env) slotCount(divisionID, gameID int) int {
	n := me.DB.DivisionUmpiresNeeded(divisionID)
	if gameID != 0 {
		for _, s := range me.DB.GameUmpires(gameID) {
			if s.Slot > n {
				n = s.Slot
			}
		}
	}
	return n
}

// slotBlock builds the picker for a game form. game is nil on the create form.
// The registered-umpire list is withheld for draft tournaments (see canViewCertificate).
func (me *Env) slotBlock(t mydb.Tournament, divisionID int, game *mydb.Game) slotBlock {
	gid := 0
	if game != nil {
		gid = game.ID
	}
	var existing []mydb.UmpireSlot
	if gid != 0 {
		existing = me.DB.GameUmpires(gid)
	}
	var opts []umpireOption
	have := map[int]bool{}
	if t.Status != "draft" {
		verified, flagged, now := me.DB.VerifiedUmpireIDs(t.ID), me.DB.ExpiredFlagUmpireIDs(t.ID), time.Now()
		for _, u := range me.DB.ListUmpires() {
			o := umpireOption{UserID: u.UserID, Label: u.Name}
			if u.Expired(flagged[u.UserID], now) { // warn only; expired umpires stay assignable
				o.Label += " ⚠ expired"
			} else if verified[u.UserID] {
				o.Label += " ✔ verified"
			}
			o.Blocked = game != nil && me.DB.UmpireConflict(u.UserID, game.Start, gid)
			opts = append(opts, o)
			have[u.UserID] = true
		}
	}
	var slots []slotView
	n := me.slotCount(divisionID, gid)
	for i := 1; i <= n; i++ {
		sv := slotView{Slot: i}
		for _, s := range existing {
			if s.Slot != i {
				continue
			}
			sv.Status, sv.FreeName = s.Status, s.FreeName
			if s.Status == "declined" {
				sv.DeclinedBy = s.UserName
			} else if s.UserID > 0 {
				sv.UserID = s.UserID
				if !have[s.UserID] { // keep current holder selectable even if no longer listed
					opts = append(opts, umpireOption{UserID: s.UserID, Label: s.UserName + " (not currently listed)"})
					have[s.UserID] = true
				}
			}
		}
		slots = append(slots, sv)
	}
	return slotBlock{Slots: slots, Umpires: opts}
}

// parseSlotChoices reads ump_user_N / ump_name_N for N=1..n. A chosen user wins over a typed name.
func parseSlotChoices(r *http.Request, n int) []slotChoice {
	out := make([]slotChoice, 0, n)
	for i := 1; i <= n; i++ {
		c := slotChoice{Slot: i}
		if uid, err := strconv.Atoi(r.FormValue(fmt.Sprintf("ump_user_%d", i))); err == nil && uid > 0 {
			c.UserID = uid
		} else {
			c.Name = strings.TrimSpace(r.FormValue(fmt.Sprintf("ump_name_%d", i)))
		}
		out = append(out, c)
	}
	return out
}

// validateSlotsFor is validateSlots plus the draft-tournament rule: draft tournaments may only use free-form names.
func (me *Env) validateSlotsFor(t mydb.Tournament, choices []slotChoice, start time.Time, gameID int) string {
	if t.Status == "draft" {
		for _, c := range choices {
			if c.UserID > 0 {
				return "Registered umpires can be assigned once the tournament is approved and published."
			}
		}
	}
	return me.validateSlots(choices, start, gameID)
}

// validateSlots returns a user-facing error ("" when ok). It does not write.
func (me *Env) validateSlots(choices []slotChoice, start time.Time, gameID int) string {
	held := map[int]bool{} // users already holding a slot on this game
	if gameID != 0 {
		for _, s := range me.DB.GameUmpires(gameID) {
			if s.UserID > 0 {
				held[s.UserID] = true
			}
		}
	}
	seen := map[int]bool{}
	for _, c := range choices {
		if c.UserID == 0 {
			continue
		}
		u, isUmpire := me.DB.GetUmpire(c.UserID)
		if !isUmpire && !held[c.UserID] {
			return "Selected user is not a registered umpire."
		}
		name := u.Name
		if !isUmpire {
			if au, err := me.DB.GetUserByID(c.UserID); err == nil {
				name = au.Name
			}
		}
		if seen[c.UserID] {
			return name + " is selected for more than one slot."
		}
		seen[c.UserID] = true
		if me.DB.UmpireConflict(c.UserID, start, gameID) {
			return name + " is already assigned to another game that overlaps this time."
		}
	}
	return ""
}

// applySlots writes the chosen slots for g and emails registered umpires who were assigned or removed.
// It returns a user-facing error ("" when every assignment succeeded). Slots whose holder changes are
// cleared first, so an umpire can be moved or swapped between slots of the same game.
func (me *Env) applySlots(g mydb.Game, choices []slotChoice) string {
	byslot := map[int]slotChoice{}
	chosen := map[int]bool{} // registered users named anywhere in the submission
	for _, c := range choices {
		byslot[c.Slot] = c
		if c.UserID > 0 {
			chosen[c.UserID] = true
		}
	}
	line := gameLine(g)
	old := map[int]mydb.UmpireSlot{}
	for _, s := range me.DB.GameUmpires(g.ID) {
		c, ok := byslot[s.Slot]
		if !ok {
			continue
		}
		old[s.Slot] = s
		keep := false
		switch {
		case s.UserID > 0 && s.Status == "declined":
			keep = c.UserID == 0 && c.Name == "" && !chosen[s.UserID] // declined record stays until refilled
		case s.UserID > 0:
			keep = c.UserID == s.UserID
		default:
			keep = c.UserID == 0 && c.Name == s.FreeName
		}
		if keep {
			continue
		}
		me.DB.ClearUmpireSlot(g.ID, s.Slot)
		delete(old, s.Slot)
		if s.UserID > 0 && s.Status != "declined" {
			me.notifyUser(s.UserID, "Removed from a game", "You are no longer assigned to: "+line, "/umpire")
		}
	}
	var failed []string
	for _, c := range choices {
		if c.UserID == 0 && c.Name == "" {
			continue
		}
		if o, had := old[c.Slot]; had && ((c.UserID > 0 && o.UserID == c.UserID) || (c.UserID == 0 && o.UserID == 0 && o.FreeName == c.Name)) {
			continue // unchanged
		}
		if err := me.DB.AssignUmpire(g.ID, c.Slot, c.UserID, c.Name); err != nil {
			slogAssignErr(g.ID, c.Slot, err)
			failed = append(failed, fmt.Sprintf("slot %d: %v", c.Slot, err))
			continue
		}
		if c.UserID > 0 {
			me.notifyUser(c.UserID, "You have been scheduled to umpire", "Please accept or decline: "+line, "/umpire")
		}
	}
	if len(failed) > 0 {
		return "Some umpire assignments could not be saved (" + strings.Join(failed, "; ") + ")."
	}
	return ""
}

// notifyGameChanged re-requests confirmation when a game's time or place changes.
func (me *Env) notifyGameChanged(old, updated mydb.Game) {
	if old.Start.Equal(updated.Start) && old.Location == updated.Location {
		return
	}
	for _, s := range me.DB.ResetUmpireSlotsPending(updated.ID) {
		me.notifyUser(s.UserID, "Game changed — please re-confirm",
			"This game has changed. Was: "+gameLine(old)+". Now: "+gameLine(updated)+". Please accept or decline again.", "/umpire")
	}
	for _, s := range me.DB.ResetRoleSlotsPending(updated.ID) {
		me.notifyUser(s.UserID, "Game changed — please re-confirm",
			"This game has changed ("+s.RoleName+"). Was: "+gameLine(old)+". Now: "+gameLine(updated)+". Please accept or decline again.", "/umpire")
	}
}

// notifyGameDeleted must be called before the game is deleted.
func (me *Env) notifyGameDeleted(g mydb.Game) {
	for _, s := range me.DB.GameUmpires(g.ID) {
		if s.UserID > 0 && s.Status != "declined" {
			me.notifyUser(s.UserID, "Game cancelled", "This game was cancelled: "+gameLine(g), "/umpire")
		}
	}
	for _, s := range me.DB.GameRoleSlots(g.ID) {
		if s.UserID > 0 && s.Status != "declined" {
			me.notifyUser(s.UserID, "Game cancelled", "This game was cancelled: "+gameLine(g), "/umpire")
		}
	}
}

func slogAssignErr(gameID, slot int, err error) {
	slog.Error("assign umpire", "game", gameID, "slot", slot, "err", err)
}
