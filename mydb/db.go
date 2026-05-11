package mydb

import "time"

// DB is the database interface implemented by both *MyDB and *FakeDB.
type DB interface {
	// Tournaments
	AddTournament(name, sport, location, notes string, date time.Time) int
	ReturnTournaments() []Tournament
	ReturnTournamentByID(id int) Tournament
	ReturnTournamentsComingUp() []Tournament
	ReturnTournamentsRecent() []Tournament
	ReturnTournamentsFuture(page int) ([]Tournament, int)
	ReturnTournamentsPast(page int) ([]Tournament, int)
	DelTournament(id int)
	UpdateTournament(id int, name, sport, location, notes string, date time.Time)

	// Divisions
	AddDivision(tournamentID int, name string)
	DelDivision(id int)
	UpdateDivision(id int, name string)
	ReturnDivisions(tournamentID int) []Division
	ReturnDivisionByID(id int) Division

	// Teams
	AddTeam(tournamentID, divisionID int, name, coach string)
	DelTeam(id int)
	ReturnTeamsByTournamentID(tournamentID int) []Team
	UpdateTeam(id, divisionID int, name, coach string)
	ReturnTeamsByDivisionID(divisionID int) []Team
	ReturnTeamsByDivisionIDWithStats(divisionID int) []Team
	ReturnTeamByID(id int) Team
	TeamWins(id int) int
	TeamLosses(id int) int
	TeamScoredFor(id int) int
	TeamScoredAgainst(id int) int
	GamesPlayedByTeam(id int) int

	// Games
	AddGame(tournamentID, divisionID, homeTeamID, awayTeamID int, location, startTime, umpire string)
	AllGamesByDivision(divisionID int) []Game
	AllGamesByTeam(teamID int) []Game
	ReturnGameByID(gameID int) Game
	DelGame(id int)
	UpdateGame(id, divisionID, homeTeamID, awayTeamID int, location, startTime, umpire string)
	AllGames(tournamentID int) []Game
	ScoreGame(gid, hscore, ascore int)
	DeleteTeamScore(gameID int)
	AddTeamScore(tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore int)
	DidTeamABeatTeamB(teamAID, teamBID int) (bool, bool)
	IsGameScored(id int) bool

	// Users
	CountUsers() int
	CreateUser(email, name, passwordHash, verificationToken string, emailVerified, isAdmin bool) (User, error)
	GetUserByEmail(email string) (User, error)
	GetUserByID(id int) (User, error)
	GetUserByVerificationToken(token string) (User, error)
	GetUserByResetToken(token string) (User, error)
	MarkEmailVerified(userID int)
	SetVerificationToken(userID int, token string)
	SetResetToken(userID int, token string, expires time.Time)
	UpdatePassword(userID int, passwordHash string)

	// Roles
	GetRolesForUser(userID int) []TournamentRole
	GetRolesForTournament(tournamentID int) []TournamentRole
	AssignRole(userID, tournamentID int, role string, teamID int) error
	RemoveRole(userID, tournamentID int)
	CreateInvitation(email string, tournamentID int, role string, teamID int, token string, expires time.Time) error
	GetInvitationByEmail(email string) (Invitation, bool)
	GetInvitationByToken(token string) (Invitation, bool)
	DeleteInvitation(id int)

	// Login rate limiting
	GetLoginAttempt(ip string) (LoginAttempt, error)
	RecordLoginFailure(ip string) error
	ClearLoginAttempts(ip string) error
	PruneLoginAttempts() error
}

// Compile-time checks that both implementations satisfy the interface.
var _ DB = (*MyDB)(nil)
var _ DB = (*FakeDB)(nil)
