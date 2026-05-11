package main

import (
	"encoding/hex"
	_ "embed"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/csrf"
	"github.com/julienschmidt/httprouter"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/webhandler"
)

//go:embed Banner.png
var defaultBanner []byte

//go:embed style.css
var defaultCSS []byte

var banner []byte
var favico []byte

func main() {
	cfg := LoadConfig("tourneyweb.conf")
	spew.Dump(cfg)
	db := mydb.New(cfg.Database, cfg.Debug)
	email := webhandler.NewEmailService(
		cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword,
		cfg.FromEmail, cfg.BaseURL,
	)
	wh := webhandler.New(db, email, cfg.DisableDelete, cfg.DisableEmailVerification)
	log.Println("Listening on port:", cfg.Port)
	LoadBanner(cfg.BannerImagePath)
	LoadFavico()

	router := httprouter.New()
	// Health check
	router.GET("/healthz", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusOK)
	})
	// Static assets
	router.GET("/style.css", PrintCSS)
	router.GET("/favicon.ico", PrintFavIco)
	router.GET("/img/topimage.jpg", PrintBannerLogo)
	// Auth routes
	router.GET("/login", wh.LoginForm)
	router.POST("/login", wh.Login)
	router.GET("/register", wh.RegisterForm)
	router.POST("/register", wh.Register)
	router.GET("/verify", wh.VerifyEmail)
	router.POST("/resend-verification", wh.ResendVerification)
	router.GET("/password-reset", wh.PasswordResetForm)
	router.POST("/password-reset", wh.PasswordReset)
	router.GET("/password-reset/confirm", wh.PasswordResetConfirmForm)
	router.POST("/password-reset/confirm", wh.PasswordResetConfirm)
	router.POST("/logout", wh.Logout)
	// Public routes
	router.GET("/", wh.TournamentList)
	router.GET("/hrderbyinfo", wh.PrintHRDerby)
	router.GET("/tournaments/:tid", wh.TournamentHome)
	router.GET("/tournaments/:tid/divisions/:did", wh.PrintDivision)
	router.GET("/tournaments/:tid/teams/:teamid", wh.ShowTeam)
	router.GET("/tournaments/:tid/games", wh.Games)
	// Score routes (directors + staff)
	router.GET("/tournaments/:tid/score/games/:gid", wh.ScoreGame)
	router.POST("/tournaments/:tid/score/games/:gid", wh.RecordScore)
	// Director manage routes
	router.GET("/tournaments/:tid/manage/roles", wh.ManageRoles)
	router.POST("/tournaments/:tid/manage/roles", wh.AssignRole)
	router.POST("/tournaments/:tid/manage/invite", wh.InviteUser)
	router.POST("/tournaments/:tid/manage/roles/:uid/remove", wh.RemoveRole)
	// Admin routes
	router.GET("/admin/tournaments", wh.AdminTournaments)
	router.POST("/admin/tournaments", wh.CreateTournament)
	router.GET("/admin/tournaments/:tid", wh.AdminTournamentView)
	router.GET("/admin/tournaments/:tid/divisions", wh.AddDivisionForm)
	router.POST("/admin/tournaments/:tid/divisions", wh.AddDivisionForm)
	router.GET("/admin/tournaments/:tid/divisions/:did", wh.AdminDivisionView)
	router.POST("/admin/tournaments/:tid/divisions/:did/delete", wh.DeleteDivision)
	router.GET("/admin/tournaments/:tid/teams", wh.Teams)
	router.POST("/admin/tournaments/:tid/teams", wh.Teams)
	router.POST("/admin/tournaments/:tid/teams/:teamid/delete", wh.DeleteTeam)
	router.GET("/admin/tournaments/:tid/games", wh.AdminGames)
	router.GET("/admin/tournaments/:tid/divisions/:did/games/new", wh.CreateGame)
	router.POST("/admin/tournaments/:tid/games", wh.CreateGameSubmit)
	router.GET("/admin/tournaments/:tid/games/:gid/score", wh.ScoreGame)
	router.POST("/admin/tournaments/:tid/games/:gid/score", wh.RecordScore)
	router.POST("/admin/tournaments/:tid/games/:gid/delete", wh.DelGame)
	router.GET("/admin/tournaments/:tid/edit", wh.EditTournament)
	router.POST("/admin/tournaments/:tid/edit", wh.EditTournament)
	router.GET("/admin/tournaments/:tid/divisions/:did/edit", wh.EditDivision)
	router.POST("/admin/tournaments/:tid/divisions/:did/edit", wh.EditDivision)
	router.GET("/admin/tournaments/:tid/teams/:teamid/edit", wh.EditTeam)
	router.POST("/admin/tournaments/:tid/teams/:teamid/edit", wh.EditTeam)
	router.GET("/admin/tournaments/:tid/games/:gid/edit", wh.EditGame)
	router.POST("/admin/tournaments/:tid/games/:gid/edit", wh.EditGame)

	csrfKey := decodeCSRFKey(cfg.CSRFKey)
	csrfMiddleware := csrf.Protect(csrfKey, csrf.Secure(false))
	log.Fatal(http.ListenAndServe(":"+cfg.Port, wh.RequestLogger(csrfMiddleware(router))))
}

func PrintFavIco(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if len(favico) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-type", mime.TypeByExtension(".ico"))
	w.Write(favico)
}

func PrintBannerLogo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", http.DetectContentType(banner))
	w.Write(banner)
}

func PrintCSS(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-type", mime.TypeByExtension(".css"))
	w.Write(defaultCSS)
}

func LoadBanner(path string) {
	banner = defaultBanner
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Println("banner: could not load", path, err)
		return
	}
	banner = data
}

func LoadFavico() {
	favico, _ = os.ReadFile("favicon.ico")
}

func decodeCSRFKey(hexKey string) []byte {
	if hexKey == "" {
		log.Fatalf("csrfkey config: not set — generate with: openssl rand -hex 32")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		log.Fatalf("csrfkey config: must be a valid hex string: %v", err)
	}
	if len(key) != 32 {
		log.Fatalf("csrfkey config: must decode to exactly 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	return key
}
