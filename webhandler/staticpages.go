package webhandler

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) HelpPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "help", helpData{baseData: newBase(r)})
}

func (me *Env) AboutPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "about", aboutData{baseData: newBase(r)})
}
