package main

import (
	"encoding/hex"
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

var banner []byte
var favico []byte

func main() {

	cfg := LoadConfig("tourneyweb.conf")
	spew.Dump(cfg)
	db := mydb.New(cfg.Database, cfg.Debug)
	wh := webhandler.New(db, cfg.AdminPassword, cfg.DisableDelete)
	log.Println(cfg.Port)
	LoadBanner(cfg.BannerImagePath)
	LoadFavico()

	router := httprouter.New()
	// Static assets
	router.GET("/style.css", PrintCSS)
	router.GET("/favicon.ico", PrintFavIco)
	router.GET("/img/topimage.jpg", PrintBannerLogo)
	// Public routes
	router.GET("/", wh.TournamentList)
	router.GET("/login", wh.LoginForm)
	router.POST("/login", wh.Login)
	router.GET("/hrderbyinfo", wh.PrintHRDerby)
	router.GET("/tournaments/:tid", wh.TournamentHome)
	router.GET("/tournaments/:tid/divisions/:did", wh.PrintDivision)
	router.GET("/tournaments/:tid/teams/:teamid", wh.ShowTeam)
	router.GET("/tournaments/:tid/games", wh.Games)
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
	router.POST("/admin/tournaments/:tid/teams/delete", wh.DeleteTeam)
	router.GET("/admin/tournaments/:tid/games", wh.AdminGames)
	router.GET("/admin/tournaments/:tid/divisions/:did/games/new", wh.CreateGame)
	router.POST("/admin/tournaments/:tid/games", wh.CreateGameSubmit)
	router.GET("/admin/tournaments/:tid/games/:gid/score", wh.ScoreGame)
	router.POST("/admin/tournaments/:tid/games/:gid/score", wh.RecordScore)
	router.GET("/admin/tournaments/:tid/games/:gid/delete", wh.DelGame)
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

	w.Header().Set("Content-type", mime.TypeByExtension(".ico"))
	w.Write(favico)
}

func PrintBannerLogo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	w.Header().Set("Content-type", mime.TypeByExtension(".jpg"))
	w.Write(banner)
}

func PrintCSS(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	css := `
a {
	color:#2a2a2a;
	text-decoration:none;
}
a, img {
	border:none;
	outline:none
	
}
a:hover {
	color:#2a2a2a;
	
}
	`
	w.Header().Set("Content-type", mime.TypeByExtension(".css"))
	w.Write([]byte(css))
}

func LoadBanner(path string) {
	var err error
	banner, err = os.ReadFile(path)
	if err != nil {
		log.Println("File doesn't exist", err)
	}

	//if file doesn't exist lets put something here
}

func LoadFavico() {
	var err error
	favico, err = os.ReadFile("favicon.ico")
	if err != nil {
		log.Println("File doesn't exist", err)
	}
	//if file doesn't exist lets put something here
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
