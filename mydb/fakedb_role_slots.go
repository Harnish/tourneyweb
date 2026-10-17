package mydb

import (
	"errors"
	"sort"
	"strings"
)

func (f *FakeDB) roleSlotsLocked(match func(RoleSlot) bool) []RoleSlot {
	var out []RoleSlot
	for _, s := range f.roleSlots {
		if !match(s) {
			continue
		}
		for _, r := range f.extraRoles {
			if r.ID == s.RoleID {
				s.RoleName = r.Name
			}
		}
		if s.UserID > 0 {
			s.UserName, s.UserEmail = f.users[s.UserID].Name, f.users[s.UserID].Email
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GameID != out[j].GameID {
			return out[i].GameID < out[j].GameID
		}
		if a, b := strings.ToLower(out[i].RoleName), strings.ToLower(out[j].RoleName); a != b {
			return a < b
		}
		if out[i].RoleID != out[j].RoleID {
			return out[i].RoleID < out[j].RoleID
		}
		return out[i].Slot < out[j].Slot
	})
	return out
}

func (f *FakeDB) GameRoleSlots(gameID int) []RoleSlot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.roleSlotsLocked(func(s RoleSlot) bool { return s.GameID == gameID })
}

func (f *FakeDB) RoleSlotsForUser(userID int) []RoleSlot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.roleSlotsLocked(func(s RoleSlot) bool { return s.UserID == userID })
}

// heldOnGameLocked: user holds a non-declined role slot on the game, other than exceptRole/exceptSlot.
func (f *FakeDB) heldRoleOnGameLocked(userID, gameID, exceptRole, exceptSlot int) bool {
	for _, s := range f.roleSlots {
		if s.GameID == gameID && s.UserID == userID && s.Status != "declined" && !(s.RoleID == exceptRole && s.Slot == exceptSlot) {
			return true
		}
	}
	return false
}

func (f *FakeDB) AssignRoleSlot(gameID, roleID, slot, userID int, freeName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	status := "accepted"
	if userID > 0 {
		for _, s := range f.umpSlots {
			if s.GameID == gameID && s.UserID == userID && s.Status != "declined" {
				return ErrUmpireDuplicate
			}
		}
		if f.heldRoleOnGameLocked(userID, gameID, roleID, slot) {
			return ErrUmpireDuplicate
		}
		if f.conflictLocked(userID, f.games[gameID].Start, gameID) {
			return ErrUmpireConflict
		}
		status, freeName = "pending", ""
	}
	for i, s := range f.roleSlots {
		if s.GameID == gameID && s.RoleID == roleID && s.Slot == slot {
			f.roleSlots[i].UserID, f.roleSlots[i].FreeName, f.roleSlots[i].Status = userID, freeName, status
			return nil
		}
	}
	f.roleSlots = append(f.roleSlots, RoleSlot{ID: f.newID(), GameID: gameID, RoleID: roleID, Slot: slot, UserID: userID, FreeName: freeName, Status: status})
	return nil
}

func (f *FakeDB) ClearRoleSlot(gameID, roleID, slot int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.roleSlots[:0]
	for _, s := range f.roleSlots {
		if !(s.GameID == gameID && s.RoleID == roleID && s.Slot == slot) {
			kept = append(kept, s)
		}
	}
	f.roleSlots = kept
}

func (f *FakeDB) RespondRoleSlot(gameID, roleID, slot, userID int, accept bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.roleSlots {
		if s.GameID == gameID && s.RoleID == roleID && s.Slot == slot && s.UserID == userID && userID > 0 {
			if s.Status == "declined" {
				return ErrUmpireSlotDeclined
			}
			f.roleSlots[i].Status = "declined"
			if accept {
				f.roleSlots[i].Status = "accepted"
			}
			return nil
		}
	}
	return errors.New("slot not assigned to this user")
}

func (f *FakeDB) ResetRoleSlotsPending(gameID int) []RoleSlot {
	f.mu.Lock()
	defer f.mu.Unlock()
	match := func(s RoleSlot) bool { return s.GameID == gameID && s.UserID > 0 && s.Status == "accepted" }
	affected := f.roleSlotsLocked(match)
	for i, s := range f.roleSlots {
		if match(s) {
			f.roleSlots[i].Status = "pending"
		}
	}
	return affected
}
