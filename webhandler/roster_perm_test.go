package webhandler

import "testing"

func TestIsCoachForTeam(t *testing.T) {
	user := TWUser{
		ID: 1,
		Roles: []TournamentRole{
			{TournamentID: 10, Role: "coach", TeamID: 42},
		},
	}
	if !user.IsCoachForTeam(42) {
		t.Error("want true for team 42, got false")
	}
	if user.IsCoachForTeam(99) {
		t.Error("want false for team 99, got true")
	}
	noRoles := TWUser{ID: 2}
	if noRoles.IsCoachForTeam(42) {
		t.Error("want false for user with no roles")
	}
}
