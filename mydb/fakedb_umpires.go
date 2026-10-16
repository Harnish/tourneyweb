package mydb

import "sort"

type fakeUmpireProfile struct {
	isUmpire  bool
	certURL   string
	image     []byte
	imageType string
}

func (f *FakeDB) SetUmpireProfile(userID int, isUmpire bool, certURL string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.umpires[userID]
	p.isUmpire, p.certURL = isUmpire, certURL
	f.umpires[userID] = p
}

func (f *FakeDB) SetUmpireCertImage(userID int, data []byte, contentType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.umpires[userID]
	p.image, p.imageType = append([]byte(nil), data...), contentType
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
	return Umpire{UserID: userID, Name: u.Name, Email: u.Email, CertURL: p.certURL, HasCertImage: len(p.image) > 0}, true
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
