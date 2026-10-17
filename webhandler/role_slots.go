package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

type roleChoice struct {
	RoleID, Slot, UserID int
	Name                 string
}

type roleSlotView struct {
	Slot       int
	UserID     int
	Display    string // registered user's name or the free-form name
	Status     string
	DeclinedBy string
}

type roleSlotGroup struct {
	RoleID int
	Name   string
	Slots  []roleSlotView
}

// roleSlotBlock is the data for the "roleSlots" template used by the manage game forms.
type roleSlotBlock struct {
	SearchURL string
	Roles     []roleSlotGroup
}

// roleSlotCounts is how many slots each role shows on a game form: the division's count, or more if the game already uses higher slots.
func (me *Env) roleSlotCounts(divisionID, gameID int) map[int]int {
	counts := map[int]int{}
	for rid, n := range me.DB.DivisionRoleNeeds(divisionID) {
		counts[rid] = n
	}
	if gameID != 0 {
		for _, s := range me.DB.GameRoleSlots(gameID) {
			if s.Slot > counts[s.RoleID] {
				counts[s.RoleID] = s.Slot
			}
		}
	}
	return counts
}

func (me *Env) roleSlotBlock(t mydb.Tournament, divisionID int, game *mydb.Game) roleSlotBlock {
	gid := 0
	var existing []mydb.RoleSlot
	if game != nil {
		gid = game.ID
		existing = me.DB.GameRoleSlots(gid)
	}
	counts := me.roleSlotCounts(divisionID, gid)
	block := roleSlotBlock{SearchURL: fmt.Sprintf("/tournaments/%d/manage/users/search", t.ID)}
	for _, role := range me.DB.TournamentExtraRoles(t.ID) {
		n := counts[role.ID]
		if n == 0 {
			continue
		}
		grp := roleSlotGroup{RoleID: role.ID, Name: role.Name}
		for i := 1; i <= n; i++ {
			sv := roleSlotView{Slot: i}
			for _, s := range existing {
				if s.RoleID != role.ID || s.Slot != i {
					continue
				}
				sv.Status = s.Status
				switch {
				case s.Status == "declined":
					sv.DeclinedBy = s.UserName
				case s.UserID > 0:
					sv.UserID, sv.Display = s.UserID, s.UserName
				default:
					sv.Display = s.FreeName
				}
			}
			grp.Slots = append(grp.Slots, sv)
		}
		block.Roles = append(block.Roles, grp)
	}
	return block
}

// parseRoleChoices reads role_user_<role>_<slot> / role_name_<role>_<slot>. A chosen user wins over a typed name.
func (me *Env) parseRoleChoices(r *http.Request, divisionID, gameID int) []roleChoice {
	counts := me.roleSlotCounts(divisionID, gameID)
	var rids []int
	for rid := range counts {
		rids = append(rids, rid)
	}
	sort.Ints(rids)
	var out []roleChoice
	for _, rid := range rids {
		for i := 1; i <= counts[rid]; i++ {
			c := roleChoice{RoleID: rid, Slot: i}
			suffix := fmt.Sprintf("%d_%d", rid, i)
			if uid, err := strconv.Atoi(r.FormValue("role_user_" + suffix)); err == nil && uid > 0 {
				c.UserID = uid
			} else {
				c.Name = strings.TrimSpace(r.FormValue("role_name_" + suffix))
			}
			out = append(out, c)
		}
	}
	return out
}

// validateRoleChoices returns a user-facing error ("" when ok). It does not write.
// ump are the same submission's umpire choices: nobody may hold two slots on one game.
func (me *Env) validateRoleChoices(t mydb.Tournament, choices []roleChoice, ump []slotChoice, start time.Time, gameID int) string {
	seen := map[int]bool{}
	for _, c := range ump {
		if c.UserID > 0 {
			seen[c.UserID] = true
		}
	}
	for _, c := range choices {
		if c.UserID == 0 {
			continue
		}
		if t.Status == "draft" {
			return "Registered users can be assigned once the tournament is approved and published."
		}
		u, err := me.DB.GetUserByID(c.UserID)
		if err != nil || u.ID == 0 {
			return "Selected user was not found."
		}
		if seen[c.UserID] {
			return u.Name + " is selected for more than one slot."
		}
		seen[c.UserID] = true
		if me.DB.UmpireConflict(c.UserID, start, gameID) {
			return u.Name + " is already assigned to another game that overlaps this time."
		}
	}
	return ""
}

type roleKey struct{ role, slot int }

// clearRoleChoices clears role slots whose holder changes or is blanked in the submission and tells removed users.
// It returns the slots that stay. Callers run it before applySlots so a holder can move from a role to an umpire slot;
// applyRoleChoices runs it again afterwards, which is then a no-op.
func (me *Env) clearRoleChoices(g mydb.Game, choices []roleChoice) map[roleKey]mydb.RoleSlot {
	byslot := map[roleKey]roleChoice{}
	chosen := map[int]bool{}
	for _, c := range choices {
		byslot[roleKey{c.RoleID, c.Slot}] = c
		if c.UserID > 0 {
			chosen[c.UserID] = true
		}
	}
	line := gameLine(g)
	old := map[roleKey]mydb.RoleSlot{}
	for _, s := range me.DB.GameRoleSlots(g.ID) {
		k := roleKey{s.RoleID, s.Slot}
		c, ok := byslot[k]
		if !ok {
			continue
		}
		old[k] = s
		var keep bool
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
		me.DB.ClearRoleSlot(g.ID, s.RoleID, s.Slot)
		delete(old, k)
		if s.UserID > 0 && s.Status != "declined" {
			me.notifyUser(s.UserID, "Removed from a game", "You are no longer assigned as "+s.RoleName+" to: "+line, "/umpire")
		}
	}
	return old
}

// applyRoleChoices writes the chosen role slots for g and notifies registered users who were assigned or removed,
// following the same keep/clear/assign rules as applySlots.
func (me *Env) applyRoleChoices(g mydb.Game, choices []roleChoice) string {
	old := me.clearRoleChoices(g, choices)
	line := gameLine(g)
	var failed []string
	for _, c := range choices {
		if c.UserID == 0 && c.Name == "" {
			continue
		}
		if o, had := old[roleKey{c.RoleID, c.Slot}]; had && ((c.UserID > 0 && o.UserID == c.UserID) || (c.UserID == 0 && o.UserID == 0 && o.FreeName == c.Name)) {
			continue // unchanged
		}
		if err := me.DB.AssignRoleSlot(g.ID, c.RoleID, c.Slot, c.UserID, c.Name); err != nil {
			slog.Error("assign role slot", "game", g.ID, "role", c.RoleID, "slot", c.Slot, "err", err)
			failed = append(failed, fmt.Sprintf("%s slot %d: %v", me.DB.ExtraRoleByID(c.RoleID).Name, c.Slot, err))
			continue
		}
		if c.UserID > 0 {
			me.notifyUser(c.UserID, "You have been scheduled for a role",
				"Please accept or decline "+me.DB.ExtraRoleByID(c.RoleID).Name+" for: "+line, "/umpire")
		}
	}
	if len(failed) > 0 {
		return "Some role assignments could not be saved (" + strings.Join(failed, "; ") + ")."
	}
	return ""
}
