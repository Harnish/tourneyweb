package mydb

import (
	"errors"
	"sort"
	"strings"
	"time"
)

type fakeUmpireProfile struct {
	isUmpire  bool
	certURL   string
	image     []byte
	imageType string
	linkedAt  time.Time
	expiresOn time.Time
}

func (f *FakeDB) SetUmpireProfile(userID int, isUmpire bool, certURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.umpires[userID]
	if p.certURL != certURL { // a new link restarts the certification
		p.linkedAt = time.Now()
		if certURL == "" && len(p.image) == 0 {
			p.linkedAt = time.Time{}
		}
		f.clearExpiredFlagsLocked(userID)
	}
	p.isUmpire, p.certURL = isUmpire, certURL
	f.umpires[userID] = p
}

func (f *FakeDB) clearExpiredFlagsLocked(userID int) {
	for k := range f.umpExpired {
		if k[1] == userID {
			delete(f.umpExpired, k)
		}
	}
}

func (f *FakeDB) SetUmpireCertExpiry(userID int, expiresOn time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.umpires[userID]
	p.expiresOn = expiresOn
	f.umpires[userID] = p
}

func (f *FakeDB) SetUmpireExpiredFlag(tournamentID, userID, byUserID int, flagged bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := [2]int{tournamentID, userID}
	if flagged {
		f.umpExpired[k] = true
	} else {
		delete(f.umpExpired, k)
	}
}

func (f *FakeDB) ExpiredFlagUmpireIDs(tournamentID int) map[int]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]bool{}
	for k := range f.umpExpired {
		if k[0] == tournamentID {
			out[k[1]] = true
		}
	}
	return out
}

func (f *FakeDB) SetUmpireCertImage(userID int, data []byte, contentType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.umpires[userID]
	p.image, p.imageType = append([]byte(nil), data...), contentType
	if len(data) > 0 { // a new image restarts the certification
		p.linkedAt = time.Now()
		f.clearExpiredFlagsLocked(userID)
	}
	f.umpires[userID] = p
}

func (f *FakeDB) GetUmpireCertImage(userID int) ([]byte, string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.umpires[userID]
	if len(p.image) == 0 {
		return nil, "", false
	}
	return p.image, p.imageType, true
}

func (f *FakeDB) umpireLocked(userID int) (Umpire, bool) {
	p := f.umpires[userID]
	u, ok := f.users[userID]
	if !p.isUmpire || !ok {
		return Umpire{}, false
	}
	return Umpire{UserID: userID, Name: u.Name, Email: u.Email, CertURL: p.certURL, HasCertImage: len(p.image) > 0, LinkedAt: p.linkedAt, ExpiresOn: p.expiresOn}, true
}

func (f *FakeDB) GetUmpire(userID int) (Umpire, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.umpireLocked(userID)
}

func (f *FakeDB) ListUmpires() []Umpire {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Umpire
	for id := range f.umpires {
		if u, ok := f.umpireLocked(id); ok {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (f *FakeDB) SetUmpireVerified(tournamentID, userID, byUserID int, verified bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := [2]int{tournamentID, userID}
	if verified {
		f.umpVerified[k] = true
	} else {
		delete(f.umpVerified, k)
	}
}

func (f *FakeDB) VerifiedUmpireIDs(tournamentID int) map[int]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]bool{}
	for k := range f.umpVerified {
		if k[0] == tournamentID {
			out[k[1]] = true
		}
	}
	return out
}

func (f *FakeDB) DivisionUmpiresNeeded(divisionID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if n, ok := f.umpNeeded[divisionID]; ok {
		return n
	}
	return 1
}

func (f *FakeDB) SetDivisionUmpiresNeeded(divisionID, n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.umpNeeded[divisionID] = clampUmpiresNeeded(n)
}

// --- Slots ---

func (f *FakeDB) slotsLocked(match func(UmpireSlot) bool) []UmpireSlot {
	var out []UmpireSlot
	for _, s := range f.umpSlots {
		if match(s) {
			if s.UserID > 0 {
				s.UserName, s.UserEmail = f.users[s.UserID].Name, f.users[s.UserID].Email
			}
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GameID != out[j].GameID {
			return out[i].GameID < out[j].GameID
		}
		return out[i].Slot < out[j].Slot
	})
	return out
}

func (f *FakeDB) GameUmpires(gameID int) []UmpireSlot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.slotsLocked(func(s UmpireSlot) bool { return s.GameID == gameID })
}

func (f *FakeDB) UmpireSlotsForUser(userID int) []UmpireSlot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.slotsLocked(func(s UmpireSlot) bool { return s.UserID == userID })
}

func (f *FakeDB) conflictLocked(userID int, start time.Time, excludeGameID int) bool {
	if start.IsZero() {
		return false
	}
	for _, s := range f.umpSlots {
		if s.UserID != userID || s.Status == "declined" || s.GameID == excludeGameID {
			continue
		}
		if g, ok := f.games[s.GameID]; ok && !g.Start.IsZero() && UmpireWindowsOverlap(start, g.Start) {
			return true
		}
	}
	return false
}

func (f *FakeDB) UmpireConflict(userID int, start time.Time, excludeGameID int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conflictLocked(userID, start, excludeGameID)
}

func (f *FakeDB) refreshUmpireCacheLocked(gameID int) {
	g, ok := f.games[gameID]
	if !ok {
		return
	}
	var names []string
	for _, s := range f.slotsLocked(func(s UmpireSlot) bool { return s.GameID == gameID && s.Status != "declined" }) {
		if s.UserID > 0 {
			names = append(names, s.UserName)
		} else {
			names = append(names, s.FreeName)
		}
	}
	g.Umpire = strings.Join(names, ", ")
	f.games[gameID] = g
}

func (f *FakeDB) AssignUmpire(gameID, slot, userID int, freeName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if userID > 0 {
		for _, s := range f.umpSlots {
			if s.GameID == gameID && s.UserID == userID && s.Slot != slot {
				return ErrUmpireDuplicate
			}
		}
		if f.conflictLocked(userID, f.games[gameID].Start, gameID) {
			return ErrUmpireConflict
		}
	}
	status := "accepted"
	if userID > 0 {
		status = "pending"
		freeName = ""
	}
	for i, s := range f.umpSlots {
		if s.GameID == gameID && s.Slot == slot {
			f.umpSlots[i].UserID, f.umpSlots[i].FreeName, f.umpSlots[i].Status = userID, freeName, status
			f.refreshUmpireCacheLocked(gameID)
			return nil
		}
	}
	f.umpSlots = append(f.umpSlots, UmpireSlot{ID: f.newID(), GameID: gameID, Slot: slot, UserID: userID, FreeName: freeName, Status: status})
	f.refreshUmpireCacheLocked(gameID)
	return nil
}

func (f *FakeDB) ClearUmpireSlot(gameID, slot int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.umpSlots[:0]
	for _, s := range f.umpSlots {
		if !(s.GameID == gameID && s.Slot == slot) {
			kept = append(kept, s)
		}
	}
	f.umpSlots = kept
	f.refreshUmpireCacheLocked(gameID)
}

func (f *FakeDB) RespondUmpireSlot(gameID, slot, userID int, accept bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.umpSlots {
		if s.GameID == gameID && s.Slot == slot && s.UserID == userID && userID > 0 {
			if s.Status == "declined" {
				return ErrUmpireSlotDeclined
			}
			f.umpSlots[i].Status = "declined"
			if accept {
				f.umpSlots[i].Status = "accepted"
			}
			f.refreshUmpireCacheLocked(gameID)
			return nil
		}
	}
	return errors.New("slot not assigned to this user")
}

func (f *FakeDB) ResetUmpireSlotsPending(gameID int) []UmpireSlot {
	f.mu.Lock()
	defer f.mu.Unlock()
	affected := f.slotsLocked(func(s UmpireSlot) bool { return s.GameID == gameID && s.UserID > 0 && s.Status == "accepted" })
	for i, s := range f.umpSlots {
		if s.GameID == gameID && s.UserID > 0 && s.Status == "accepted" {
			f.umpSlots[i].Status = "pending"
		}
	}
	return affected
}
