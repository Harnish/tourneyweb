package webhandler

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func (f *roleFx) searchIDs(t *testing.T, q, division string) []int {
	t.Helper()
	req := httptest.NewRequest("GET", "/x?q="+q+"&division="+division, nil)
	req = req.WithContext(ctxWithUser(req.Context(), f.director))
	w := httptest.NewRecorder()
	f.env.UserSearch(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}})
	var got []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, w.Body.String())
	}
	var ids []int
	for _, h := range got {
		if len(h) != 2 || h["id"] == nil || h["name"] == nil {
			t.Fatalf("JSON shape must stay {id,name}: %v", h)
		}
		ids = append(ids, int(h["id"].(float64)))
	}
	return ids
}

func (f *roleFx) user(name string) int {
	u, _ := f.db.CreateUser(name+"@x.com", name, "h", "", true, false)
	return u.ID
}

func TestUserSearchListsFollowersFirst(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	abe, zed, tour := f.user("Pat Abe"), f.user("Pat Zed"), f.user("Pat Mid")
	f.db.AddFollow(zed, "division", f.did)
	f.db.AddFollow(tour, "tournament", f.tid)
	got := f.searchIDs(t, "pat", strconv.Itoa(f.did))
	if len(got) != 3 || got[0] != tour || got[1] != zed || got[2] != abe {
		t.Fatalf("want followers (alphabetical) then the rest: got %v (tour %d zed %d abe %d)", got, tour, zed, abe)
	}
}

func TestUserSearchFollowerBeyondLimitStillListed(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	for i := 0; i < 11; i++ {
		f.user("Pat A" + strconv.Itoa(100+i))
	}
	zzz := f.user("Pat Zzz")
	f.db.AddFollow(zzz, "division", f.did)
	got := f.searchIDs(t, "pat", strconv.Itoa(f.did))
	if len(got) != 10 || got[0] != zzz {
		t.Fatalf("follower must lead a full page: %v", got)
	}
}

func TestUserSearchDedupesFollowers(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	u := f.user("Pat Both")
	f.db.AddFollow(u, "division", f.did)
	f.db.AddFollow(u, "tournament", f.tid)
	got := f.searchIDs(t, "pat", strconv.Itoa(f.did))
	if len(got) != 1 {
		t.Fatalf("listed twice: %v", got)
	}
}

func TestUserSearchForeignDivisionIgnored(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	other := f.db.AddTournament("T2", "baseball", "L", "", f.game.Start, "published")
	od := f.db.AddDivision(other, "12U")
	abe, zed := f.user("Pat Abe"), f.user("Pat Zed")
	f.db.AddFollow(zed, "division", od) // follows a level of some other tournament
	got := f.searchIDs(t, "pat", strconv.Itoa(od))
	if len(got) != 2 || got[0] != abe {
		t.Fatalf("a foreign division must not rank anyone: %v", got)
	}
	if got := f.searchIDs(t, "pat", "abc"); len(got) != 2 || got[0] != abe {
		t.Fatalf("bad division: %v", got)
	}
}

func TestRoleSlotSearchCarriesDivision(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	blk := f.env.roleSlotBlock(f.db.ReturnTournamentByID(f.tid), f.did, nil)
	if blk.DivisionID != f.did {
		t.Fatalf("division: %d", blk.DivisionID)
	}
}
