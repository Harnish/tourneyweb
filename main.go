package main

import (
	"context"
	"encoding/hex"
	_ "embed"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/csrf"
	"github.com/julienschmidt/httprouter"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/webhandler"
)

//go:embed Banner.png
var defaultBanner []byte

//go:embed style.css
var defaultCSS []byte

//go:embed favicon.ico
var defaultFavico []byte

var banner []byte

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := LoadConfig("tourneyweb.conf")
	db := mydb.New(cfg.Database, cfg.Debug)
	email := webhandler.NewEmailService(
		cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword,
		cfg.FromEmail, cfg.BaseURL,
	)
	wh := webhandler.New(db, email, cfg.DisableDelete, cfg.DisableEmailVerification)
	slog.Info("listening", "port", cfg.Port)
	LoadBanner(cfg.BannerImagePath)

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
	// Self-service tournament creation (requires login, enforced in handler)
	// Must be registered before /tournaments/:tid to avoid "new" matching as :tid
	router.GET("/tournaments/new", wh.NewTournamentForm)
	router.POST("/tournaments/new", wh.NewTournament)
	router.GET("/tournaments/:tid", wh.TournamentHome)
	router.GET("/tournaments/:tid/divisions/:did", wh.PrintDivision)
	router.GET("/tournaments/:tid/teams/:teamid", wh.ShowTeam)
	router.GET("/tournaments/:tid/games", wh.Games)
	router.GET("/tournaments/:tid/extras", wh.TournamentExtras)
	router.GET("/map", wh.MapView)
	// Score routes (directors + staff)
	router.GET("/tournaments/:tid/score/games/:gid", wh.ScoreGame)
	router.POST("/tournaments/:tid/score/games/:gid", wh.RecordScore)
	// Director manage routes
	router.GET("/tournaments/:tid/manage/publish", wh.ManagePublish)
	router.POST("/tournaments/:tid/manage/publish", wh.ManagePublish)
	router.GET("/tournaments/:tid/manage", wh.ManageDashboard)
	router.GET("/tournaments/:tid/manage/extras", wh.ManageExtras)
	router.POST("/tournaments/:tid/manage/extras", wh.ManageExtras)
	router.GET("/tournaments/:tid/manage/roles", wh.ManageRoles)
	router.POST("/tournaments/:tid/manage/roles", wh.AssignRole)
	router.POST("/tournaments/:tid/manage/invite", wh.InviteUser)
	router.POST("/tournaments/:tid/manage/roles/:uid/remove", wh.RemoveRole)
	// Admin routes
	router.GET("/admin/tournaments", wh.AdminTournaments)
	router.POST("/admin/tournaments", wh.CreateTournament)
	// queue must be registered before :tid to avoid "queue" matching as :tid
	router.GET("/admin/tournaments/queue", wh.TournamentQueue)
	router.POST("/admin/tournaments/:tid/issue-code", wh.IssueCode)
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
	router.POST("/admin/tournaments/:tid/divisions/:did/games/generate", wh.GenerateGames)
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
	// Admin location routes
	router.GET("/admin/locations", wh.Locations)
	router.POST("/admin/locations", wh.Locations)
	router.GET("/admin/locations/:lid/edit", wh.EditLocation)
	router.POST("/admin/locations/:lid/edit", wh.EditLocation)
	router.POST("/admin/locations/:lid/delete", wh.DeleteLocation)

	csrfKey := decodeCSRFKey(cfg.CSRFKey)
	csrfMiddleware := csrf.Protect(csrfKey, csrf.Secure(false))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: wh.RequestLogger(csrfMiddleware(router)),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func PrintFavIco(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-type", mime.TypeByExtension(".ico"))
	w.Write(defaultFavico)
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
		slog.Warn("banner: could not load", "path", path, "err", err)
		return
	}
	banner = data
}


func decodeCSRFKey(hexKey string) []byte {
	if hexKey == "" {
		slog.Error("csrfkey config: not set — generate with: openssl rand -hex 32")
		os.Exit(1)
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		slog.Error("csrfkey config: must be a valid hex string", "err", err)
		os.Exit(1)
	}
	if len(key) != 32 {
		slog.Error("csrfkey config: must decode to exactly 32 bytes (64 hex chars)", "got", len(key))
		os.Exit(1)
	}
	return key
}
