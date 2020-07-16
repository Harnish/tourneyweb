package main

import (
	"io/ioutil"
	"log"
	"mime"
	"net/http"

	"github.com/davecgh/go-spew/spew"
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

	router := httprouter.New()
	router.GET("/", wh.PrintIndex)
	router.GET("/login", wh.LoginForm)
	router.POST("/login", wh.Login)
	router.GET("/divisions/:id", wh.PrintDivision)
	router.GET("/teams/:teamid", wh.ShowTeam)
	router.GET("/games", wh.Games)
	router.GET("/style.css", PrintCSS)
	router.GET("/favicon.ico", PrintFavIco)
	router.GET("/img/topimage.jpg", PrintBannerLogo)
	router.GET("/hrderbyinfo", wh.PrintHRDerby)
	router.GET("/admin/", wh.AdminIndex)
	router.GET("/admin/adddivisionform", wh.AddDivisionForm)
	router.POST("/admin/adddivision", wh.AddDivisionForm)
	router.POST("/admin/deldivision", wh.AddDivisionForm)
	router.GET("/admin/teams", wh.Teams)
	router.POST("/admin/addteam", wh.Teams)
	router.GET("/admin/creategame/:divisionid", wh.CreateGame)
	router.POST("/admin/addgame", wh.CreateGameSubmit)
	router.GET("/admin/delgame/:gameid", wh.DelGame)
	router.GET("/admin/scoregame/:gameid", wh.ScoreGame)
	router.GET("/admin/games", wh.AdminGames)
	router.POST("/admin/scoregamepost", wh.RecordScore)
	router.GET("/admin/divisions/:divisionid", wh.AdminDivisionView)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, wh.RequestLogger(router)))
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
	banner, err = ioutil.ReadFile(path)
	if err != nil {
		log.Println("File doesn't exist", err)
	}

	//if file doesn't exist lets put something here
}

func LoadFavico() {
	var err error
	favico, err = ioutil.ReadFile("favicon.ico")
	if err != nil {
		log.Println("File doesn't exist", err)
	}
	//if file doesn't exist lets put something here
}
