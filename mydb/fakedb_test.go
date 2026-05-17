package mydb_test

import (
	"testing"
	"time"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func TestFakeDB_TournamentCRUD(t *testing.T) {
	db := mydb.NewFakeDB()
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	id := db.AddTournament("Summer Classic", "baseball", "City Park", "notes", date, "published")
	if id == 0 {
		t.Fatal("expected non-zero tournament ID")
	}
	got := db.ReturnTournamentByID(id)
	if got.Name != "Summer Classic" {
		t.Errorf("name: got %q, want %q", got.Name, "Summer Classic")
	}
	db.UpdateTournament(id, "Summer Classic II", "baseball", "City Park", "notes", date)
	if db.ReturnTournamentByID(id).Name != "Summer Classic II" {
		t.Error("update did not persist")
	}
	db.DelTournament(id)
	if db.ReturnTournamentByID(id).ID != 0 {
		t.Error("delete did not remove tournament")
	}
}

func TestFakeDB_ScoringAndStats(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")
	db.AddDivision(tid, "12U")
	divs := db.ReturnDivisions(tid)
	if len(divs) != 1 {
		t.Fatalf("expected 1 division, got %d", len(divs))
	}
	did := divs[0].ID

	db.AddTeam(tid, did, "Red", "Coach A")
	db.AddTeam(tid, did, "Blue", "Coach B")
	teams := db.ReturnTeamsByDivisionID(did)
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
	red, blue := teams[0], teams[1]

	db.AddGame(tid, did, red.ID, blue.ID, "Field 1", time.Time{}, "")
	games := db.AllGamesByDivision(did)
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	gid := games[0].ID

	if db.IsGameScored(gid) {
		t.Error("game should not be scored yet")
	}

	db.ScoreGame(gid, 5, 3) // red wins
	if !db.IsGameScored(gid) {
		t.Error("game should be scored after ScoreGame")
	}

	if db.TeamWins(red.ID) != 1 {
		t.Errorf("red wins: got %d, want 1", db.TeamWins(red.ID))
	}
	if db.TeamLosses(red.ID) != 0 {
		t.Errorf("red losses: got %d, want 0", db.TeamLosses(red.ID))
	}
	if db.TeamWins(blue.ID) != 0 {
		t.Errorf("blue wins: got %d, want 0", db.TeamWins(blue.ID))
	}
	if db.TeamLosses(blue.ID) != 1 {
		t.Errorf("blue losses: got %d, want 1", db.TeamLosses(blue.ID))
	}
	if db.TeamScoredFor(red.ID) != 5 {
		t.Errorf("red runs for: got %d, want 5", db.TeamScoredFor(red.ID))
	}
	if db.TeamScoredAgainst(red.ID) != 3 {
		t.Errorf("red runs against: got %d, want 3", db.TeamScoredAgainst(red.ID))
	}

	played, won := db.DidTeamABeatTeamB(red.ID, blue.ID)
	if !played || !won {
		t.Errorf("DidTeamABeatTeamB(red, blue): played=%v won=%v, want true true", played, won)
	}
	played, won = db.DidTeamABeatTeamB(blue.ID, red.ID)
	if !played || won {
		t.Errorf("DidTeamABeatTeamB(blue, red): played=%v won=%v, want true false", played, won)
	}

	withStats := db.ReturnTeamsByDivisionIDWithStats(did)
	for _, team := range withStats {
		if team.ID == red.ID && team.Wins != 1 {
			t.Errorf("WithStats red wins: got %d, want 1", team.Wins)
		}
	}
}

func TestFakeDB_UserCRUD(t *testing.T) {
	db := mydb.NewFakeDB()

	if db.CountUsers() != 0 {
		t.Error("expected 0 users initially")
	}

	u, err := db.CreateUser("alice@example.com", "Alice", "hash", "tok1", false, false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID == 0 || u.Email != "alice@example.com" {
		t.Errorf("unexpected user: %+v", u)
	}

	_, err = db.CreateUser("alice@example.com", "Alice2", "hash2", "", false, false)
	if err == nil {
		t.Error("expected duplicate email error")
	}

	got, err := db.GetUserByEmail("alice@example.com")
	if err != nil || got.ID != u.ID {
		t.Errorf("GetUserByEmail: %v %+v", err, got)
	}

	got, err = db.GetUserByVerificationToken("tok1")
	if err != nil || got.ID != u.ID {
		t.Errorf("GetUserByVerificationToken: %v %+v", err, got)
	}

	db.MarkEmailVerified(u.ID)
	got, _ = db.GetUserByID(u.ID)
	if !got.EmailVerified || got.VerificationToken != "" {
		t.Error("MarkEmailVerified did not clear token")
	}

	expires := time.Now().Add(time.Hour)
	db.SetResetToken(u.ID, "reset1", expires)
	got, err = db.GetUserByResetToken("reset1")
	if err != nil || got.ID != u.ID {
		t.Errorf("GetUserByResetToken: %v %+v", err, got)
	}

	db.UpdatePassword(u.ID, "newhash")
	got, _ = db.GetUserByID(u.ID)
	if got.PasswordHash != "newhash" || got.ResetToken != "" {
		t.Error("UpdatePassword did not clear reset token")
	}
}

func TestFakeDB_Roles(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")
	u, _ := db.CreateUser("bob@example.com", "Bob", "hash", "", true, false)

	if err := db.AssignRole(u.ID, tid, "director", 0); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	roles := db.GetRolesForUser(u.ID)
	if len(roles) != 1 || roles[0].Role != "director" {
		t.Errorf("unexpected roles: %+v", roles)
	}

	// Re-assigning same user+tournament updates in place.
	db.AssignRole(u.ID, tid, "staff", 0)
	roles = db.GetRolesForUser(u.ID)
	if len(roles) != 1 || roles[0].Role != "staff" {
		t.Errorf("re-assign expected staff, got: %+v", roles)
	}

	db.RemoveRole(u.ID, tid)
	if len(db.GetRolesForUser(u.ID)) != 0 {
		t.Error("expected no roles after RemoveRole")
	}
}

func TestFakeDB_Invitations(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")
	expires := time.Now().Add(7 * 24 * time.Hour)

	if err := db.CreateInvitation("carol@example.com", tid, "staff", 0, "token123", expires); err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}

	inv, ok := db.GetInvitationByToken("token123")
	if !ok || inv.Email != "carol@example.com" {
		t.Errorf("GetInvitationByToken: ok=%v inv=%+v", ok, inv)
	}

	inv, ok = db.GetInvitationByEmail("carol@example.com")
	if !ok || inv.Token != "token123" {
		t.Errorf("GetInvitationByEmail: ok=%v inv=%+v", ok, inv)
	}

	db.DeleteInvitation(inv.ID)
	_, ok = db.GetInvitationByToken("token123")
	if ok {
		t.Error("expected invitation to be deleted")
	}
}

func TestFakeDB_LoginAttempts(t *testing.T) {
	db := mydb.NewFakeDB()
	ip := "192.168.1.1"

	a, _ := db.GetLoginAttempt(ip)
	if a.Count != 0 {
		t.Error("expected zero count initially")
	}

	db.RecordLoginFailure(ip)
	db.RecordLoginFailure(ip)
	a, _ = db.GetLoginAttempt(ip)
	if a.Count != 2 {
		t.Errorf("expected count 2, got %d", a.Count)
	}
	if !a.LockedUntil.After(time.Now()) {
		t.Error("expected LockedUntil to be in the future")
	}

	db.ClearLoginAttempts(ip)
	a, _ = db.GetLoginAttempt(ip)
	if a.Count != 0 {
		t.Error("expected count 0 after clear")
	}
}

func TestFakeDB_VerificationCodes(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "draft")
	otherTID := db.AddTournament("T2", "baseball", "L", "", time.Now(), "draft")

	code, err := db.IssueVerificationCode(tid)
	if err != nil {
		t.Fatalf("IssueVerificationCode: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("expected 8-char code, got %q (len %d)", code, len(code))
	}

	// Wrong tournament → error
	if err := db.RedeemVerificationCode(code, otherTID); err == nil {
		t.Error("expected error for wrong tournament")
	}
	// Tournament not yet published
	if db.ReturnTournamentByID(tid).Status != "draft" {
		t.Error("should still be draft after failed redemption")
	}

	// Valid redemption
	if err := db.RedeemVerificationCode(code, tid); err != nil {
		t.Fatalf("RedeemVerificationCode: %v", err)
	}
	if db.ReturnTournamentByID(tid).Status != "published" {
		t.Error("tournament should be published after redemption")
	}

	// Cannot reuse
	if err := db.RedeemVerificationCode(code, tid); err == nil {
		t.Error("expected error for already-used code")
	}
}

func TestFakeDB_TournamentLocations(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")

	db.AddLocation("Global Field", "123 Main", "", 0, 0, 0)
	db.AddLocation("Home Field", "456 Oak", "", 40.0, -75.0, tid)

	all := db.GetLocations()
	if len(all) != 2 {
		t.Fatalf("GetLocations: expected 2, got %d", len(all))
	}

	byTID := db.GetLocationsByTournamentID(tid)
	if len(byTID) != 1 {
		t.Fatalf("GetLocationsByTournamentID: expected 1, got %d", len(byTID))
	}
	if byTID[0].Name != "Home Field" {
		t.Errorf("expected Home Field, got %q", byTID[0].Name)
	}
	if byTID[0].TournamentID != tid {
		t.Errorf("TournamentID: got %d, want %d", byTID[0].TournamentID, tid)
	}
}

func TestFakeDB_TournamentStatus(t *testing.T) {
	db := mydb.NewFakeDB()
	future := time.Date(2027, 8, 1, 0, 0, 0, 0, time.UTC)

	draftID := db.AddTournament("Draft T", "baseball", "City", "", future, "draft")
	pubID := db.AddTournament("Pub T", "baseball", "City", "", future, "published")

	if db.ReturnTournamentByID(draftID).Status != "draft" {
		t.Error("expected draft status")
	}
	if db.ReturnTournamentByID(pubID).Status != "published" {
		t.Error("expected published status")
	}

	drafts := db.ReturnDraftTournaments()
	foundDraft := false
	for _, d := range drafts {
		if d.ID == draftID {
			foundDraft = true
		}
		if d.ID == pubID {
			t.Error("published tournament appeared in ReturnDraftTournaments")
		}
	}
	if !foundDraft {
		t.Error("draft tournament not in ReturnDraftTournaments")
	}

	futureTourneys, _ := db.ReturnTournamentsFuture(1)
	for _, tt := range futureTourneys {
		if tt.ID == draftID {
			t.Error("draft tournament appeared in public future listing")
		}
	}

	// Verify ReturnTournamentsComingUp excludes drafts
	comingUp := db.ReturnTournamentsComingUp()
	for _, tt := range comingUp {
		if tt.ID == draftID {
			t.Error("draft tournament appeared in ReturnTournamentsComingUp")
		}
	}

	// Verify ReturnTournamentsRecent excludes drafts
	recent := db.ReturnTournamentsRecent()
	for _, tt := range recent {
		if tt.ID == draftID {
			t.Error("draft tournament appeared in ReturnTournamentsRecent")
		}
	}
}

func TestFakeDB_RulesCRUD(t *testing.T) {
	db := mydb.NewFakeDB()
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tid := db.AddTournament("Test", "baseball", "Park", "", date, "published")

	// Initially empty tournament rules
	tournament := db.ReturnTournamentByID(tid)
	if tournament.RulesHTML != "" {
		t.Errorf("expected empty rules, got %q", tournament.RulesHTML)
	}

	// Set tournament rules
	db.SetTournamentRules(tid, "<p>Tournament rules</p>")
	tournament = db.ReturnTournamentByID(tid)
	if tournament.RulesHTML != "<p>Tournament rules</p>" {
		t.Errorf("SetTournamentRules: got %q", tournament.RulesHTML)
	}

	// Add division
	db.AddDivision(tid, "12U")
	divs := db.ReturnDivisions(tid)
	if len(divs) != 1 {
		t.Fatalf("expected 1 division, got %d", len(divs))
	}
	did := divs[0].ID

	// Initially empty division rules
	div := db.ReturnDivisionByID(did)
	if div.RulesHTML != "" {
		t.Errorf("expected empty division rules, got %q", div.RulesHTML)
	}

	// Set division rules
	db.SetDivisionRules(did, "<p>Division rules</p>")
	div = db.ReturnDivisionByID(did)
	if div.RulesHTML != "<p>Division rules</p>" {
		t.Errorf("SetDivisionRules: got %q", div.RulesHTML)
	}

	// ReturnDivisions also returns RulesHTML
	divs = db.ReturnDivisions(tid)
	if divs[0].RulesHTML != "<p>Division rules</p>" {
		t.Errorf("ReturnDivisions RulesHTML: got %q", divs[0].RulesHTML)
	}

	// Tournament rules unchanged after setting division rules
	tournament = db.ReturnTournamentByID(tid)
	if tournament.RulesHTML != "<p>Tournament rules</p>" {
		t.Errorf("tournament rules unchanged: got %q", tournament.RulesHTML)
	}
}

func TestFakeDB_RankingCriteria(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")

	// AddDivision creates a division with the default criteria
	db.AddDivision(tid, "12U")
	divs := db.ReturnDivisions(tid)
	if len(divs) != 1 {
		t.Fatalf("expected 1 division, got %d", len(divs))
	}
	did := divs[0].ID

	// Default criteria should be populated (not nil/empty)
	got := db.ReturnDivisionByID(did)
	if len(got.RankingCriteria) == 0 {
		t.Error("expected non-empty default RankingCriteria")
	}
	if got.RankingCriteria[0] != "wins" {
		t.Errorf("first default criterion: got %q, want \"wins\"", got.RankingCriteria[0])
	}

	// UpdateDivision stores custom criteria
	custom := []string{"wins", "run_differential", "coin_flip"}
	db.UpdateDivision(did, "12U", custom)
	got = db.ReturnDivisionByID(did)
	if len(got.RankingCriteria) != 3 {
		t.Fatalf("after update: got %d criteria, want 3", len(got.RankingCriteria))
	}
	if got.RankingCriteria[1] != "run_differential" {
		t.Errorf("second criterion: got %q, want \"run_differential\"", got.RankingCriteria[1])
	}

	// ReturnDivisions also returns RankingCriteria
	divs = db.ReturnDivisions(tid)
	if len(divs[0].RankingCriteria) != 3 {
		t.Errorf("ReturnDivisions RankingCriteria: got %d criteria, want 3", len(divs[0].RankingCriteria))
	}
}

func TestFakeDB_NewsCRUD(t *testing.T) {
	db := mydb.NewFakeDB()

	// Site news: tournamentID = 0
	id1 := db.AddNews(0, "Site Post 1", "<p>Body 1</p>", 0)
	id2 := db.AddNews(0, "Site Post 2", "<p>Body 2</p>", 0)

	siteNews := db.GetSiteNews()
	if len(siteNews) != 2 {
		t.Fatalf("GetSiteNews: want 2, got %d", len(siteNews))
	}
	// newest first: id2 should be first
	if siteNews[0].ID != id2 {
		t.Errorf("GetSiteNews order: want id2=%d first, got id=%d", id2, siteNews[0].ID)
	}

	// Tournament news
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")
	id3 := db.AddNews(tid, "Tourney News", "<p>T body</p>", 0)
	tNews := db.GetTournamentNews(tid)
	if len(tNews) != 1 || tNews[0].ID != id3 {
		t.Errorf("GetTournamentNews: want 1 item with id=%d, got %v", id3, tNews)
	}

	// Site news unaffected by tournament news
	if len(db.GetSiteNews()) != 2 {
		t.Error("GetSiteNews should still return 2 after adding tournament news")
	}

	// GetNewsByID
	item, ok := db.GetNewsByID(id1)
	if !ok || item.Title != "Site Post 1" {
		t.Errorf("GetNewsByID: ok=%v, title=%q", ok, item.Title)
	}
	_, ok = db.GetNewsByID(99999)
	if ok {
		t.Error("GetNewsByID: expected not found for missing id")
	}

	// Update
	db.UpdateNews(id1, "Updated Title", "<p>Updated</p>")
	updated, _ := db.GetNewsByID(id1)
	if updated.Title != "Updated Title" || updated.Body != "<p>Updated</p>" {
		t.Errorf("UpdateNews: got title=%q body=%q", updated.Title, updated.Body)
	}

	// Delete
	db.DeleteNews(id1)
	_, ok = db.GetNewsByID(id1)
	if ok {
		t.Error("DeleteNews: item should be gone")
	}
	if len(db.GetSiteNews()) != 1 {
		t.Error("GetSiteNews: should have 1 item after delete")
	}
}
