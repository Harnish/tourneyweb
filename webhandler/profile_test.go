package webhandler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/Harnish/tourneyweb/mydb"
)

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)

func profileEnv(t *testing.T) (*Env, *mydb.FakeDB, TWUser) {
	t.Helper()
	db := mydb.NewFakeDB()
	u, err := db.CreateUser("u@x.com", "Una", "h", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	return &Env{DB: db}, db, TWUser{ID: u.ID, Email: u.Email, Name: u.Name}
}

func postProfile(env *Env, user TWUser, fields map[string]string, file []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if file != nil {
		fw, _ := mw.CreateFormFile("cert_image", "cert.bin")
		fw.Write(file)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/profile", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, user))
	w := httptest.NewRecorder()
	env.Profile(w, req, nil)
	return w
}

func getCert(env *Env, user TWUser, uid int) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/umpires/x/certificate", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, user))
	w := httptest.NewRecorder()
	env.UmpireCertificate(w, req, httprouter.Params{{Key: "uid", Value: strconv.Itoa(uid)}})
	return w
}

func TestCanViewCertificate(t *testing.T) {
	env, db, _ := profileEnv(t)
	pub := db.AddTournament("Pub", "baseball", "L", "", time.Now(), "published")
	draft := db.AddTournament("Draft", "baseball", "L", "", time.Now(), "draft")
	cases := []struct {
		name string
		u    TWUser
		want bool
	}{
		{"owner", TWUser{ID: 1}, true},
		{"admin", TWUser{ID: 2, IsAdmin: true}, true},
		{"director of published tournament", TWUser{ID: 5, Roles: []TournamentRole{{TournamentID: pub, Role: "director"}}}, true},
		{"director of draft tournament only", TWUser{ID: 7, Roles: []TournamentRole{{TournamentID: draft, Role: "director"}}}, false},
		{"staff", TWUser{ID: 3, Roles: []TournamentRole{{TournamentID: pub, Role: "staff"}}}, false},
		{"coach", TWUser{ID: 4, Roles: []TournamentRole{{TournamentID: pub, Role: "coach", TeamID: 1}}}, false},
		{"other user", TWUser{ID: 6}, false},
		{"anonymous", TWUser{}, false},
	}
	for _, c := range cases {
		if got := env.canViewCertificate(c.u, 1); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestCertificateHiddenAfterOptOutAndRemovable(t *testing.T) {
	env, db, owner := profileEnv(t)
	pub := db.AddTournament("Pub", "baseball", "L", "", time.Now(), "published")
	dir := TWUser{ID: 50, Roles: []TournamentRole{{TournamentID: pub, Role: "director"}}}
	db.SetUmpireProfile(owner.ID, true, "")
	db.SetUmpireCertImage(owner.ID, pngBytes, "image/png")
	if w := getCert(env, dir, owner.ID); w.Code != http.StatusOK {
		t.Fatalf("flagged umpire: got %d", w.Code)
	}
	db.SetUmpireProfile(owner.ID, false, "")
	if w := getCert(env, dir, owner.ID); w.Code != http.StatusNotFound {
		t.Errorf("unflagged umpire's certificate must be hidden from directors, got %d", w.Code)
	}
	if w := getCert(env, owner, owner.ID); w.Code != http.StatusOK {
		t.Errorf("owner can still see their own upload, got %d", w.Code)
	}
	postProfile(env, owner, map[string]string{"is_umpire": "on", "remove_cert": "on"}, nil)
	if _, _, ok := db.GetUmpireCertImage(owner.ID); ok {
		t.Error("remove_cert should delete the stored image")
	}
}

func TestCertificateAccess(t *testing.T) {
	env, db, owner := profileEnv(t)
	db.SetUmpireProfile(owner.ID, true, "")
	db.SetUmpireCertImage(owner.ID, pngBytes, "image/png")

	if w := getCert(env, TWUser{ID: owner.ID + 100}, owner.ID); w.Code != http.StatusForbidden {
		t.Errorf("non-director: got %d want 403", w.Code)
	}
	if w := getCert(env, TWUser{}, owner.ID); w.Code != http.StatusSeeOther {
		t.Errorf("anonymous: got %d want 303 redirect to login", w.Code)
	}
	w := getCert(env, owner, owner.ID)
	if w.Code != http.StatusOK || w.Header().Get("Content-Disposition") == "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("owner: code=%d headers=%v", w.Code, w.Header())
	}
	pub := db.AddTournament("Pub", "baseball", "L", "", time.Now(), "published")
	dir := TWUser{ID: 50, Roles: []TournamentRole{{TournamentID: pub, Role: "director"}}}
	if w := getCert(env, dir, owner.ID); w.Code != http.StatusOK {
		t.Errorf("director: got %d want 200", w.Code)
	}
	if w := getCert(env, owner, owner.ID+999); w.Code != http.StatusNotFound && w.Code != http.StatusForbidden {
		t.Errorf("missing umpire: got %d", w.Code)
	}
}

func TestProfileSavesFlagAndURL(t *testing.T) {
	env, db, u := profileEnv(t)
	w := postProfile(env, u, map[string]string{"is_umpire": "on", "cert_url": "https://assoc.example/una"}, pngBytes)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	got, ok := db.GetUmpire(u.ID)
	if !ok || got.CertURL != "https://assoc.example/una" || !got.HasCertImage {
		t.Fatalf("%+v ok=%v", got, ok)
	}
	// unchecking clears the flag but keeps nothing flagged
	postProfile(env, u, map[string]string{"cert_url": ""}, nil)
	if _, ok := db.GetUmpire(u.ID); ok {
		t.Fatal("unchecked box should unflag")
	}
}

func TestProfileRejectsBigUpload(t *testing.T) {
	env, db, u := profileEnv(t)
	big := append(append([]byte{}, pngBytes...), make([]byte, 6<<20)...)
	w := postProfile(env, u, map[string]string{"is_umpire": "on"}, big)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", w.Code)
	}
	if _, ok := db.GetUmpire(u.ID); ok {
		t.Fatal("profile must be unchanged on rejected upload")
	}
	if _, _, ok := db.GetUmpireCertImage(u.ID); ok {
		t.Fatal("no image stored")
	}
}

func TestProfileRejectsNonImage(t *testing.T) {
	env, db, u := profileEnv(t)
	w := postProfile(env, u, map[string]string{"is_umpire": "on"}, []byte("just some text, not an image"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", w.Code)
	}
	if _, _, ok := db.GetUmpireCertImage(u.ID); ok {
		t.Fatal("no image stored")
	}
}

func TestProfileRejectsBadURL(t *testing.T) {
	env, db, u := profileEnv(t)
	w := postProfile(env, u, map[string]string{"is_umpire": "on", "cert_url": "javascript:alert(1)"}, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", w.Code)
	}
	if _, ok := db.GetUmpire(u.ID); ok {
		t.Fatal("profile must be unchanged")
	}
}

func TestProfileRequiresLogin(t *testing.T) {
	env, _, _ := profileEnv(t)
	if w := postProfile(env, TWUser{}, map[string]string{"is_umpire": "on"}, nil); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d want 303", w.Code)
	}
}

func TestProfilePageRenders(t *testing.T) {
	env, db, u := profileEnv(t)
	db.SetUmpireProfile(u.ID, true, "https://assoc.example/una")
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, u))
	w := httptest.NewRecorder()
	env.Profile(w, req, nil)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("https://assoc.example/una")) || !bytes.Contains(w.Body.Bytes(), []byte("checked")) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestProfileSavesAndClearsExpiry(t *testing.T) {
	env, db, u := profileEnv(t)
	post := func(exp string) int {
		return postProfile(env, u, map[string]string{"is_umpire": "on", "cert_url": "https://a.example", "cert_expires": exp}, nil).Code
	}
	if c := post("2027-03-01"); c != http.StatusSeeOther {
		t.Fatalf("got %d", c)
	}
	if got, _ := db.GetUmpire(u.ID); got.ExpiresOn.Format("2006-01-02") != "2027-03-01" {
		t.Fatalf("got %v", got.ExpiresOn)
	}
	if c := post("not-a-date"); c != http.StatusBadRequest {
		t.Fatalf("invalid date: got %d want 400", c)
	}
	if got, _ := db.GetUmpire(u.ID); got.ExpiresOn.Format("2006-01-02") != "2027-03-01" {
		t.Fatal("rejected save must change nothing")
	}
	post("")
	if got, _ := db.GetUmpire(u.ID); !got.ExpiresOn.IsZero() {
		t.Fatal("blank clears the date")
	}
}

func TestProfilePageShowsLinkedDateAndExpiredNote(t *testing.T) {
	env, db, u := profileEnv(t)
	db.SetUmpireProfile(u.ID, true, "https://a.example")
	db.SetUmpireCertExpiry(u.ID, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, u))
	w := httptest.NewRecorder()
	env.Profile(w, req, nil)
	body := w.Body.String()
	if !strings.Contains(body, "Linked on "+time.Now().Format("Jan 2, 2006")) {
		t.Error("want the linked date")
	}
	if !strings.Contains(body, "expired") || !strings.Contains(body, `value="2020-01-01"`) {
		t.Errorf("want expired note and the date prefilled: %s", body)
	}
}
