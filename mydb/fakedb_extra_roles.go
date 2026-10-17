package mydb

import (
	"sort"
	"strings"
)

func (f *FakeDB) roleNameTakenLocked(tournamentID int, name string, exceptID int) bool {
	for _, r := range f.extraRoles {
		if r.TournamentID == tournamentID && r.ID != exceptID && strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}

func (f *FakeDB) AddExtraRole(tournamentID int, name string) (int, error) {
	name, err := cleanRoleName(name)
	if err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.roleNameTakenLocked(tournamentID, name, 0) {
		return 0, ErrRoleNameTaken
	}
	id := f.newID()
	f.extraRoles = append(f.extraRoles, ExtraRole{ID: id, TournamentID: tournamentID, Name: name})
	return id, nil
}

func (f *FakeDB) RenameExtraRole(tournamentID, roleID int, name string) error {
	name, err := cleanRoleName(name)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.roleNameTakenLocked(tournamentID, name, roleID) {
		return ErrRoleNameTaken
	}
	for i, r := range f.extraRoles {
		if r.ID == roleID && r.TournamentID == tournamentID {
			f.extraRoles[i].Name = name
		}
	}
	return nil
}

func (f *FakeDB) delRoleLocked(roleID int) {
	kept := f.extraRoles[:0]
	for _, r := range f.extraRoles {
		if r.ID != roleID {
			kept = append(kept, r)
		}
	}
	f.extraRoles = kept
	for k := range f.roleNeeds {
		if k[1] == roleID {
			delete(f.roleNeeds, k)
		}
	}
	slots := f.roleSlots[:0]
	for _, s := range f.roleSlots {
		if s.RoleID != roleID {
			slots = append(slots, s)
		}
	}
	f.roleSlots = slots
}

func (f *FakeDB) DelExtraRole(tournamentID, roleID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.extraRoles {
		if r.ID == roleID && r.TournamentID == tournamentID {
			f.delRoleLocked(roleID)
		}
	}
}

func (f *FakeDB) TournamentExtraRoles(tournamentID int) []ExtraRole {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ExtraRole
	for _, r := range f.extraRoles {
		if r.TournamentID == tournamentID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (f *FakeDB) ExtraRoleByID(roleID int) ExtraRole {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.extraRoles {
		if r.ID == roleID {
			return r
		}
	}
	return ExtraRole{}
}

func (f *FakeDB) DivisionRoleNeeds(divisionID int) map[int]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]int{}
	for k, n := range f.roleNeeds {
		if k[0] == divisionID && n > 0 {
			out[k[1]] = n
		}
	}
	return out
}

func (f *FakeDB) SetDivisionRoleNeeded(divisionID, roleID, n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n = clampRoleNeeded(n)
	if n == 0 {
		delete(f.roleNeeds, [2]int{divisionID, roleID})
		return
	}
	f.roleNeeds[[2]int{divisionID, roleID}] = n
}

func (f *FakeDB) SearchUsers(q string, limit int) []User {
	q = strings.ToLower(strings.TrimSpace(q))
	if len([]rune(q)) < 2 || limit <= 0 {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []User
	for _, u := range f.users {
		if strings.Contains(strings.ToLower(u.Name), q) || strings.Contains(strings.ToLower(u.Email), q) {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
