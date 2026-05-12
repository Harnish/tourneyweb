package mydb

import (
	"database/sql"
	"errors"
	"sync"
	"time"
)

// FakeDB is an in-memory implementation of DB for use in tests.
type FakeDB struct {
	mu            sync.Mutex
	nextID        int
	tournaments   map[int]Tournament
	divisions     map[int]Division
	teams         map[int]Team
	games         map[int]Game
	gamesByTeam   []gameByTeamRow
	users         map[int]User
	roles         []TournamentRole
	invitations   map[int]Invitation
	locations     map[int]Location
	loginAttempts map[string]LoginAttempt
	verifCodes    []fakeVerifCode
}

type gameByTeamRow struct {
	id            int
	tournamentID  int
	divisionID    int
	teamID        int
	opponentID    int
	gameID        int
	teamScore     int
	opponentScore int
}

type fakeVerifCode struct {
	id           int
	tournamentID int
	code         string
	redeemed     bool
}

func NewFakeDB() *FakeDB {
	return &FakeDB{
		nextID:        1,
		tournaments:   make(map[int]Tournament),
		divisions:     make(map[int]Division),
		teams:         make(map[int]Team),
		games:         make(map[int]Game),
		users:         make(map[int]User),
		invitations:   make(map[int]Invitation),
		locations:     make(map[int]Location),
		loginAttempts: make(map[string]LoginAttempt),
	}
}

func (f *FakeDB) newID() int {
	id := f.nextID
	f.nextID++
	return id
}

// --- Tournaments ---

func (f *FakeDB) AddTournament(name, sport, location, notes string, date time.Time, status string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.tournaments[id] = Tournament{ID: id, Name: name, Sport: sport, Location: location, Notes: notes, StartDate: date, Status: status}
	return id
}

func (f *FakeDB) ReturnTournaments() []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Tournament, 0, len(f.tournaments))
	for _, t := range f.tournaments {
		out = append(out, t)
	}
	return out
}

func (f *FakeDB) ReturnTournamentByID(id int) Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tournaments[id]
}

func (f *FakeDB) ReturnTournamentsComingUp() []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	cutoff := now.AddDate(0, 0, 7)
	var out []Tournament
	for _, t := range f.tournaments {
		if t.Status == "published" && !t.StartDate.Before(now) && !t.StartDate.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

func (f *FakeDB) ReturnTournamentsRecent() []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	cutoff := now.AddDate(0, 0, -7)
	var out []Tournament
	for _, t := range f.tournaments {
		if t.Status == "published" && !t.StartDate.Before(cutoff) && t.StartDate.Before(now) {
			out = append(out, t)
		}
	}
	return out
}

func (f *FakeDB) ReturnTournamentsFuture(page int) ([]Tournament, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := time.Now().AddDate(0, 0, 7)
	var all []Tournament
	for _, t := range f.tournaments {
		if t.Status == "published" && t.StartDate.After(cutoff) {
			all = append(all, t)
		}
	}
	if page < 1 {
		page = 1
	}
	return paginate(all, page)
}

func (f *FakeDB) ReturnTournamentsPast(page int) ([]Tournament, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := time.Now().AddDate(0, 0, -7)
	var all []Tournament
	for _, t := range f.tournaments {
		if t.Status == "published" && t.StartDate.Before(cutoff) {
			all = append(all, t)
		}
	}
	if page < 1 {
		page = 1
	}
	return paginate(all, page)
}

func (f *FakeDB) ReturnDraftTournaments() []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Tournament
	for _, t := range f.tournaments {
		if t.Status == "draft" {
			out = append(out, t)
		}
	}
	return out
}

func paginate(items []Tournament, page int) ([]Tournament, int) {
	total := len(items)
	start := (page - 1) * 20
	if start >= total {
		return nil, total
	}
	end := start + 20
	if end > total {
		end = total
	}
	return items[start:end], total
}

func (f *FakeDB) DelTournament(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.tournaments, id)
}

func (f *FakeDB) UpdateTournament(id int, name, sport, location, notes string, date time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tournaments[id]; ok {
		t.Name = name
		t.Sport = sport
		t.Location = location
		t.Notes = notes
		t.StartDate = date
		f.tournaments[id] = t
	}
}

func (f *FakeDB) SetTournamentExtras(id int, html string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tournaments[id]; ok {
		t.ExtrasHTML = html
		f.tournaments[id] = t
	}
}

// --- Divisions ---

func (f *FakeDB) AddDivision(tournamentID int, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.divisions[id] = Division{ID: id, TournamentID: tournamentID, Name: name}
}

func (f *FakeDB) DelDivision(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.divisions, id)
}

func (f *FakeDB) UpdateDivision(id int, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.divisions[id]; ok {
		d.Name = name
		f.divisions[id] = d
	}
}

func (f *FakeDB) ReturnDivisions(tournamentID int) []Division {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Division
	for _, d := range f.divisions {
		if d.TournamentID == tournamentID {
			out = append(out, d)
		}
	}
	return out
}

func (f *FakeDB) ReturnDivisionByID(id int) Division {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.divisions[id]
}

// --- Teams ---

func (f *FakeDB) AddTeam(tournamentID, divisionID int, name, coach string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.teams[id] = Team{
		ID:           id,
		TournamentID: tournamentID,
		Division:     f.divisions[divisionID],
		Name:         name,
		Coach:        coach,
	}
}

func (f *FakeDB) DelTeam(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.teams, id)
}

func (f *FakeDB) ReturnTeamsByTournamentID(tournamentID int) []Team {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Team
	for _, t := range f.teams {
		if t.TournamentID == tournamentID {
			out = append(out, t)
		}
	}
	return out
}

func (f *FakeDB) UpdateTeam(id, divisionID int, name, coach string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.teams[id]; ok {
		t.Division = f.divisions[divisionID]
		t.Name = name
		t.Coach = coach
		f.teams[id] = t
	}
}

func (f *FakeDB) ReturnTeamsByDivisionID(divisionID int) []Team {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Team
	for _, t := range f.teams {
		if t.Division.ID == divisionID {
			out = append(out, t)
		}
	}
	return out
}

// ReturnTeamsByDivisionIDWithStats inlines stat computation to avoid re-acquiring the mutex.
func (f *FakeDB) ReturnTeamsByDivisionIDWithStats(divisionID int) []Team {
	f.mu.Lock()
	defer f.mu.Unlock()
	var teams []Team
	for _, t := range f.teams {
		if t.Division.ID == divisionID {
			teams = append(teams, t)
		}
	}
	for i := range teams {
		id := teams[i].ID
		for _, r := range f.gamesByTeam {
			if r.teamID == id {
				if r.teamScore > r.opponentScore {
					teams[i].Wins++
				} else if r.teamScore < r.opponentScore {
					teams[i].Losses++
				}
				teams[i].RunsFor += r.teamScore
				teams[i].RunsAgainst += r.opponentScore
			}
		}
	}
	return teams
}

func (f *FakeDB) ReturnTeamByID(id int) Team {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.teams[id]
}

func (f *FakeDB) TeamWins(id int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.gamesByTeam {
		if r.teamID == id && r.teamScore > r.opponentScore {
			n++
		}
	}
	return n
}

func (f *FakeDB) TeamLosses(id int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.gamesByTeam {
		if r.teamID == id && r.teamScore < r.opponentScore {
			n++
		}
	}
	return n
}

func (f *FakeDB) TeamScoredFor(id int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.gamesByTeam {
		if r.teamID == id {
			n += r.teamScore
		}
	}
	return n
}

func (f *FakeDB) TeamScoredAgainst(id int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.gamesByTeam {
		if r.teamID == id {
			n += r.opponentScore
		}
	}
	return n
}

func (f *FakeDB) GamesPlayedByTeam(id int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.gamesByTeam {
		if r.teamID == id {
			n++
		}
	}
	return n
}

// --- Games ---

func (f *FakeDB) AddGame(tournamentID, divisionID, homeTeamID, awayTeamID int, location string, startTime time.Time, umpire string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.games[id] = Game{
		ID:           id,
		TournamentID: tournamentID,
		Division:     f.divisions[divisionID],
		HomeTeam:     f.teams[homeTeamID],
		AwayTeam:     f.teams[awayTeamID],
		Location:     location,
		Start:        startTime,
		Umpire:       umpire,
	}
}

func (f *FakeDB) AllGamesByDivision(divisionID int) []Game {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Game
	for _, g := range f.games {
		if g.Division.ID == divisionID {
			out = append(out, g)
		}
	}
	return out
}

func (f *FakeDB) AllGamesByTeam(teamID int) []Game {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Game
	for _, g := range f.games {
		if g.HomeTeam.ID == teamID || g.AwayTeam.ID == teamID {
			out = append(out, g)
		}
	}
	return out
}

func (f *FakeDB) ReturnGameByID(gameID int) Game {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.games[gameID]
}

func (f *FakeDB) DelGame(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.games, id)
}

func (f *FakeDB) UpdateGame(id, divisionID, homeTeamID, awayTeamID int, location string, startTime time.Time, umpire string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[id]
	if !ok {
		return
	}
	g.Division = f.divisions[divisionID]
	g.HomeTeam = f.teams[homeTeamID]
	g.AwayTeam = f.teams[awayTeamID]
	g.Location = location
	g.Start = startTime
	g.Umpire = umpire
	f.games[id] = g
	if g.Scored {
		f.removeGamesByTeamLocked(id)
		f.gamesByTeam = append(f.gamesByTeam,
			gameByTeamRow{id: f.newID(), tournamentID: g.TournamentID, divisionID: g.Division.ID, teamID: g.HomeTeam.ID, opponentID: g.AwayTeam.ID, gameID: id, teamScore: g.HomeScore, opponentScore: g.AwayScore},
			gameByTeamRow{id: f.newID(), tournamentID: g.TournamentID, divisionID: g.Division.ID, teamID: g.AwayTeam.ID, opponentID: g.HomeTeam.ID, gameID: id, teamScore: g.AwayScore, opponentScore: g.HomeScore},
		)
	}
}

func (f *FakeDB) AllGames(tournamentID int) []Game {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Game
	for _, g := range f.games {
		if g.TournamentID == tournamentID {
			out = append(out, g)
		}
	}
	return out
}

func (f *FakeDB) ScoreGame(gid, hscore, ascore int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[gid]
	if !ok {
		return
	}
	g.HomeScore = hscore
	g.AwayScore = ascore
	g.Scored = true
	f.games[gid] = g
	f.removeGamesByTeamLocked(gid)
	f.gamesByTeam = append(f.gamesByTeam,
		gameByTeamRow{id: f.newID(), tournamentID: g.TournamentID, divisionID: g.Division.ID, teamID: g.HomeTeam.ID, opponentID: g.AwayTeam.ID, gameID: gid, teamScore: hscore, opponentScore: ascore},
		gameByTeamRow{id: f.newID(), tournamentID: g.TournamentID, divisionID: g.Division.ID, teamID: g.AwayTeam.ID, opponentID: g.HomeTeam.ID, gameID: gid, teamScore: ascore, opponentScore: hscore},
	)
}

func (f *FakeDB) DeleteTeamScore(gameID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removeGamesByTeamLocked(gameID)
	if g, ok := f.games[gameID]; ok {
		g.Scored = false
		g.HomeScore = 0
		g.AwayScore = 0
		f.games[gameID] = g
	}
}

func (f *FakeDB) AddTeamScore(tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gamesByTeam = append(f.gamesByTeam, gameByTeamRow{
		id: f.newID(), tournamentID: tournamentID, divisionID: divisionID,
		teamID: teamID, opponentID: opponentID, gameID: gameID,
		teamScore: teamScore, opponentScore: opponentScore,
	})
}

func (f *FakeDB) DidTeamABeatTeamB(teamAID, teamBID int) (bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.gamesByTeam {
		if r.teamID == teamAID && r.opponentID == teamBID {
			return true, r.teamScore > r.opponentScore
		}
	}
	return false, false
}

func (f *FakeDB) IsGameScored(id int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.gamesByTeam {
		if r.gameID == id {
			n++
		}
	}
	return n == 2
}

func (f *FakeDB) removeGamesByTeamLocked(gameID int) {
	kept := f.gamesByTeam[:0]
	for _, r := range f.gamesByTeam {
		if r.gameID != gameID {
			kept = append(kept, r)
		}
	}
	f.gamesByTeam = kept
}

// --- Users ---

func (f *FakeDB) CountUsers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.users)
}

func (f *FakeDB) CreateUser(email, name, passwordHash, verificationToken string, emailVerified, isAdmin bool) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Email == email {
			return User{}, errors.New("duplicate email")
		}
	}
	id := f.newID()
	u := User{
		ID:                id,
		Email:             email,
		Name:              name,
		PasswordHash:      passwordHash,
		VerificationToken: verificationToken,
		EmailVerified:     emailVerified,
		IsAdmin:           isAdmin,
		CreatedAt:         time.Now(),
	}
	f.users[id] = u
	return u, nil
}

func (f *FakeDB) GetUserByEmail(email string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return User{}, sql.ErrNoRows
}

func (f *FakeDB) GetUserByID(id int) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return User{}, sql.ErrNoRows
	}
	return u, nil
}

func (f *FakeDB) GetUserByVerificationToken(token string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if token == "" {
		return User{}, sql.ErrNoRows
	}
	for _, u := range f.users {
		if u.VerificationToken == token {
			return u, nil
		}
	}
	return User{}, sql.ErrNoRows
}

func (f *FakeDB) GetUserByResetToken(token string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if token == "" {
		return User{}, sql.ErrNoRows
	}
	for _, u := range f.users {
		if u.ResetToken == token {
			return u, nil
		}
	}
	return User{}, sql.ErrNoRows
}

func (f *FakeDB) MarkEmailVerified(userID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[userID]; ok {
		u.EmailVerified = true
		u.VerificationToken = ""
		f.users[userID] = u
	}
}

func (f *FakeDB) SetVerificationToken(userID int, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[userID]; ok {
		u.VerificationToken = token
		f.users[userID] = u
	}
}

func (f *FakeDB) SetResetToken(userID int, token string, expires time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[userID]; ok {
		u.ResetToken = token
		u.ResetExpires = expires
		f.users[userID] = u
	}
}

func (f *FakeDB) UpdatePassword(userID int, passwordHash string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.users[userID]; ok {
		u.PasswordHash = passwordHash
		u.ResetToken = ""
		u.ResetExpires = time.Time{}
		f.users[userID] = u
	}
}

// --- Roles ---

func (f *FakeDB) GetRolesForUser(userID int) []TournamentRole {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []TournamentRole
	for _, r := range f.roles {
		if r.UserID == userID {
			out = append(out, r)
		}
	}
	return out
}

func (f *FakeDB) GetRolesForTournament(tournamentID int) []TournamentRole {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []TournamentRole
	for _, r := range f.roles {
		if r.TournamentID == tournamentID {
			out = append(out, r)
		}
	}
	return out
}

func (f *FakeDB) AssignRole(userID, tournamentID int, role string, teamID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, r := range f.roles {
		if r.UserID == userID && r.TournamentID == tournamentID {
			f.roles[i].Role = role
			f.roles[i].TeamID = teamID
			return nil
		}
	}
	f.roles = append(f.roles, TournamentRole{
		ID: f.newID(), UserID: userID, TournamentID: tournamentID, Role: role, TeamID: teamID,
	})
	return nil
}

func (f *FakeDB) RemoveRole(userID, tournamentID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.roles[:0]
	for _, r := range f.roles {
		if !(r.UserID == userID && r.TournamentID == tournamentID) {
			kept = append(kept, r)
		}
	}
	f.roles = kept
}

func (f *FakeDB) CreateInvitation(email string, tournamentID int, role string, teamID int, token string, expires time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.invitations[id] = Invitation{
		ID: id, Email: email, TournamentID: tournamentID,
		Role: role, TeamID: teamID, Token: token, ExpiresAt: expires,
	}
	return nil
}

func (f *FakeDB) GetInvitationByEmail(email string) (Invitation, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest Invitation
	found := false
	for _, inv := range f.invitations {
		if inv.Email == email && (!found || inv.ID > latest.ID) {
			latest = inv
			found = true
		}
	}
	return latest, found
}

func (f *FakeDB) GetInvitationByToken(token string) (Invitation, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, inv := range f.invitations {
		if inv.Token == token {
			return inv, true
		}
	}
	return Invitation{}, false
}

func (f *FakeDB) DeleteInvitation(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.invitations, id)
}

// --- Locations ---

func (f *FakeDB) AddLocation(name, address, availableFor string, lat, lng float64, tournamentID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.locations[id] = Location{ID: id, Name: name, Address: address, AvailableFor: availableFor, Latitude: lat, Longitude: lng, TournamentID: tournamentID}
	return id
}

func (f *FakeDB) GetLocations() []Location {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Location, 0, len(f.locations))
	for _, l := range f.locations {
		out = append(out, l)
	}
	return out
}

func (f *FakeDB) GetLocationByID(id int) Location {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.locations[id]
}

func (f *FakeDB) GetLocationsByTournamentID(tournamentID int) []Location {
	if tournamentID == 0 {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Location
	for _, l := range f.locations {
		if l.TournamentID == tournamentID {
			out = append(out, l)
		}
	}
	return out
}

func (f *FakeDB) UpdateLocation(id int, name, address, availableFor string, lat, lng float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if l, ok := f.locations[id]; ok {
		l.Name = name
		l.Address = address
		l.AvailableFor = availableFor
		l.Latitude = lat
		l.Longitude = lng
		f.locations[id] = l
	}
}

func (f *FakeDB) DelLocation(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.locations, id)
}

// --- Login rate limiting ---

func (f *FakeDB) GetLoginAttempt(ip string) (LoginAttempt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loginAttempts[ip], nil
}

func (f *FakeDB) RecordLoginFailure(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.loginAttempts[ip]
	a.IP = ip
	a.Count++
	delay := time.Duration(1<<min(a.Count, 8)) * time.Second
	a.LockedUntil = time.Now().Add(delay)
	f.loginAttempts[ip] = a
	return nil
}

func (f *FakeDB) ClearLoginAttempts(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.loginAttempts, ip)
	return nil
}

func (f *FakeDB) PruneLoginAttempts() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := time.Now().Add(-24 * time.Hour)
	for ip, a := range f.loginAttempts {
		if a.LockedUntil.Before(cutoff) {
			delete(f.loginAttempts, ip)
		}
	}
	return nil
}

// --- Verification codes ---

func (f *FakeDB) IssueVerificationCode(tournamentID int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	code, err := generateVerificationCode()
	if err != nil {
		return "", err
	}
	f.verifCodes = append(f.verifCodes, fakeVerifCode{id: f.newID(), tournamentID: tournamentID, code: code})
	return code, nil
}

func (f *FakeDB) RedeemVerificationCode(code string, tournamentID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, vc := range f.verifCodes {
		if vc.code == code {
			if vc.tournamentID != tournamentID {
				return errors.New("invalid code")
			}
			if vc.redeemed {
				return errors.New("code already used")
			}
			f.verifCodes[i].redeemed = true
			if t, ok := f.tournaments[tournamentID]; ok {
				t.Status = "published"
				f.tournaments[tournamentID] = t
			}
			return nil
		}
	}
	return errors.New("invalid code")
}
