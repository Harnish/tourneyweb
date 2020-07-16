package webhandler

import (
	"sort"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) SortTeams(teams []mydb.Team, sortalgo string) []mydb.Team {
	switch sortalgo {
	case "WinsHead2HeadRunsAgainstRunsEarned":
		sort.Slice(teams, func(i, j int) bool {
			//Wins
			sameWins, morewins := Wins(teams[i], teams[j])
			if sameWins {
				playedeachother, teamwin := me.DB.DidTeamABeatTeamB(teams[i].ID, teams[j].ID)
				// Need to detect loop.
				// Did they play each other
				if playedeachother {
					// Did they win?
					if teamwin {
						return true
					} else {
						return false
					}

				} else {
					// Runs against
					sameRunsAgainst, moreRunsAgainst := RunsAgainst(teams[i], teams[j])
					if sameRunsAgainst {
						// Tied
						_, moreRunsFor := RunsFor(teams[i], teams[j])
						if moreRunsFor {
							return true
						} else {
							return false
						}
					} else {
						return moreRunsAgainst
					}
				}
			} else {
				return morewins

			}
		})
	case "WinsRunsAgainstRunsEarnedHead2Head":
		sort.Slice(teams, func(i, j int) bool {
			//Wins
			if teams[i].Wins > teams[j].Wins {
				return true
			} else if teams[i].Wins < teams[j].Wins {
				return false
			} else {

				// Runs against
				if teams[i].RunsAgainst < teams[j].RunsAgainst {
					return true
				} else if teams[i].RunsAgainst > teams[j].RunsAgainst {
					return false
				} else {
					//Runs for
					if teams[i].RunsFor > teams[j].RunsFor {
						return true
					} else if teams[i].RunsFor < teams[j].RunsFor {
						return false
					} else {
						playedeachother, teamwin := me.DB.DidTeamABeatTeamB(teams[i].ID, teams[j].ID)
						// Did they play each other
						if playedeachother {
							// Did they win?
							if teamwin {
								return true
							} else {
								return false
							}

						} else {
							return false
						}
					}
				}

			}

		})
	}
	return teams

}

// RunsAgainst takes 2 teams compares their RunsAgainst.
// Returns first bool on equal, second bool is if teama is ranked above teamb
func RunsAgainst(teama, teamb mydb.Team) (bool, bool) {
	if teama.RunsAgainst < teamb.RunsAgainst {
		return false, true
	} else if teama.RunsAgainst > teamb.RunsAgainst {
		return false, false
	}
	return true, true
}

// Wins takes 2 teams and compares their wins.
// Returns first bool on equal, second bool is if teama is ranked above teamb
func Wins(teama, teamb mydb.Team) (bool, bool) {
	if teama.Wins > teamb.Wins {
		return false, true
	} else if teama.Wins < teamb.Wins {
		return false, false
	}
	return true, true
}

// RunsFor takes 2 teams and compares their RunsFor (aka runs earned).
// Returns first bool on equal, second bool is if teama is ranked above teamb
func RunsFor(teama, teamb mydb.Team) (bool, bool) {
	if teama.RunsFor > teamb.RunsFor {
		return false, true
	} else if teama.RunsFor < teamb.RunsFor {
		return false, false
	}
	return true, true
}
