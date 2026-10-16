package webhandler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
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
	dir := TWUser{ID: 5, Roles: []TournamentRole{{TournamentID: 9, Role: "director"}}}
	cases := []struct {
		name string
		u    TWUser
		want bool
	}{
		{"owner", TWUser{ID: 1}, true},
		{"admin", TWUser{ID: 2, IsAdmin: true}, true},
		{"director of any tournament", dir, true},
		{"staff", TWUser{ID: 3, Roles: []TournamentRole{{TournamentID: 9, Role: "staff"}}}, false},
		{"coach", TWUser{ID: 4, Roles: []TournamentRole{{TournamentID: 9, Role: "coach", TeamID: 1}}}, false},
		{"other user", TWUser{ID: 6}, false},
		{"anonymous", TWUser{}, false},
	}
	for _, c := range cases {
		if got := canViewCertificate(c.u, 1); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
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
	dir := TWUser{ID: 50, Roles: []TournamentRole{{TournamentID: 9, Role: "director"}}}
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
