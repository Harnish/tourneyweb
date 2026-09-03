package mydb

import "time"

// DB is the database interface implemented by both *MyDB and *FakeDB.
type DB interface {
	// Tournaments
	AddTournament(name, sport, location, notes string, date time.Time, status string) int
	ReturnTournaments() []Tournament
	ReturnTournamentByID(id int) Tournament
	ReturnTournamentsComingUp() []Tournament
	ReturnTournamentsRecent() []Tournament
	ReturnTournamentsFuture(page int) ([]Tournament, int)
	ReturnTournamentsPast(page int) ([]Tournament, int)
	ReturnDraftTournaments() []Tournament
	DelTournament(id int)
	UpdateTournament(id int, name, sport, location, notes string, date time.Time)
	SetTournamentExtras(id int, html string)
	SetTournamentRules(id int, html string)
	SetTournamentDefaultRanking(id int, criteria []string)

	// Divisions
	AddDivision(tournamentID int, name string) int
	DelDivision(id int)
	UpdateDivision(id int, name string, criteria []string)
	SetDivisionRules(id int, html string)
	ReturnDivisions(tournamentID int) []Division
	ReturnDivisionByID(id int) Division
	SetDivisionPhase(id int, phase string)

	// Teams
	AddTeam(tournamentID, divisionID int, name, coach string)
	DelTeam(id int)
	ReturnTeamsByTournamentID(tournamentID int) []Team
	UpdateTeam(id, divisionID int, name, coach string)
	SetTeamGameChanger(id int, gcURL string)
	ReturnTeamsByDivisionID(divisionID int) []Team
	ReturnTeamsByDivisionIDWithStats(divisionID int) []Team
	ReturnTeamByID(id int) Team
	TeamWins(id int) int
	TeamLosses(id int) int
	TeamScoredFor(id int) int
	TeamScoredAgainst(id int) int
	GamesPlayedByTeam(id int) int

	// Players
	AddPlayer(teamID int, number, first, last, handed, position string) int
	GetPlayersByTeamID(teamID int) []Player
	GetPlayerByID(id int) (Player, bool)
	UpdatePlayer(id int, number, first, last, handed, position string)
	DeletePlayer(id int)

	// Games
	AddGame(tournamentID, divisionID, homeTeamID, awayTeamID int, location string, startTime time.Time, umpire string) int
	AllGamesByDivision(divisionID int) []Game
	AllGamesByTeam(teamID int) []Game
	ReturnGameByID(gameID int) Game
	DelGame(id int)
	UpdateGame(id, divisionID, homeTeamID, awayTeamID int, location string, startTime time.Time, umpire string)
	AllGames(tournamentID int) []Game
	ScoreGame(gid, hscore, ascore int)
	SetGameScrimmage(gameID, teamID int)
	DeleteTeamScore(gameID int)
	AddTeamScore(tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore int)
	DidTeamABeatTeamB(teamAID, teamBID int) (bool, bool)
	RunsAllowedInHeadToHead(teamAID, teamBID int) (int, bool)
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

	// Locations
	AddLocation(name, address, availableFor string, lat, lng float64, tournamentID int) int
	GetLocations() []Location
	GetLocationByID(id int) Location
	GetLocationsByTournamentID(tournamentID int) []Location
	UpdateLocation(id int, name, address, availableFor string, lat, lng float64)
	DelLocation(id int)

	// Brackets
	CreateBracket(divisionID int, format string, size int) int
	GetBracketByID(id int) Bracket
	GetBracketByDivisionID(divisionID int) Bracket
	SetBracketStatus(bracketID int, status string)
	AddBracketSeed(bracketID, seed, teamID int)
	GetBracketSeeds(bracketID int) []BracketSeed
	UpdateBracketSeeds(bracketID int, teamIDs []int)
	AddBracketGame(bracketID, round, position int, side string) int
	SetBracketGameTeams(id, topTeamID, bottomTeamID int, topIsBye, bottomIsBye bool)
	SetBracketGameTopTeam(id, teamID int)
	SetBracketGameBottomTeam(id, teamID int)
	SetBracketGameWinner(id, winnerTeamID int)
	SetBracketGameGameID(id, gameID int)
	GetBracketGameByID(id int) BracketGame
	GetBracketGameByGameID(gameID int) BracketGame
	GetBracketGameByRoundPosition(bracketID int, side string, round, position int) BracketGame
	GetBracketGames(bracketID int) []BracketGame

	// News
	AddNews(tournamentID int, title, body string, authorID int) int
	GetSiteNews() []NewsItem
	GetTournamentNews(tournamentID int) []NewsItem
	GetNewsByID(id int) (NewsItem, bool)
	UpdateNews(id int, title, body string)
	DeleteNews(id int)

	// Login rate limiting
	GetLoginAttempt(ip string) (LoginAttempt, error)
	RecordLoginFailure(ip string) error
	ClearLoginAttempts(ip string) error
	PruneLoginAttempts() error

	// Verification codes
	IssueVerificationCode(tournamentID int) (string, error)
	RedeemVerificationCode(code string, tournamentID int) error
}

// Compile-time checks that both implementations satisfy the interface.
var _ DB = (*MyDB)(nil)
var _ DB = (*FakeDB)(nil)
