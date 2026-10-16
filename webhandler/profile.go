package webhandler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

const maxCertImageBytes = 5 << 20

// canViewCertificate: the umpire, site admins, and directors (of any tournament) may see credentials.
func canViewCertificate(u TWUser, ownerID int) bool {
	if !u.LoggedIn() {
		return false
	}
	if u.ID == ownerID || u.IsAdmin {
		return true
	}
	for _, r := range u.Roles {
		if r.Role == "director" {
			return true
		}
	}
	return false
}

func (me *Env) Profile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, maxCertImageBytes+(1<<20))
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, "Upload too large or malformed (max 5 MB)", http.StatusBadRequest)
			return
		}
		certURL := strings.TrimSpace(r.FormValue("cert_url"))
		if certURL != "" && !strings.HasPrefix(certURL, "http://") && !strings.HasPrefix(certURL, "https://") {
			http.Error(w, "Certification link must start with http:// or https://", http.StatusBadRequest)
			return
		}
		var image []byte
		var contentType string
		if file, _, err := r.FormFile("cert_image"); err == nil {
			defer file.Close()
			image, err = io.ReadAll(io.LimitReader(file, maxCertImageBytes+1))
			if err != nil || len(image) > maxCertImageBytes {
				http.Error(w, "Certificate image must be 5 MB or smaller", http.StatusBadRequest)
				return
			}
			if len(image) > 0 {
				contentType = http.DetectContentType(image)
				if contentType != "image/jpeg" && contentType != "image/png" && contentType != "application/pdf" {
					http.Error(w, "Certificate must be a JPG, PNG or PDF", http.StatusBadRequest)
					return
				}
			}
		}
		me.DB.SetUmpireProfile(user.ID, r.FormValue("is_umpire") != "", certURL)
		if len(image) > 0 {
			me.DB.SetUmpireCertImage(user.ID, image, contentType)
		}
		http.Redirect(w, r, "/profile?saved=1", http.StatusSeeOther)
		return
	}
	u, isUmpire := me.DB.GetUmpire(user.ID)
	me.render(w, "profile", profileData{
		baseData:     newBase(r),
		IsUmpire:     isUmpire,
		CertURL:      u.CertURL,
		HasCertImage: u.HasCertImage,
		Saved:        r.URL.Query().Get("saved") != "",
	})
}

func (me *Env) UmpireCertificate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	uid, err := strconv.Atoi(ps.ByName("uid"))
	if err != nil || !canViewCertificate(user, uid) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	data, contentType, ok := me.DB.GetUmpireCertImage(uid)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="certificate-%d"`, uid))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}
