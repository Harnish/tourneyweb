package webhandler

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rivo/sessions"
	"golang.org/x/crypto/bcrypt"
)

func (me *Env) RegisterForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "register", registerData{baseData: newBase(r)})
}

func (me *Env) Register(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	email := r.FormValue("email")
	name := r.FormValue("name")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	renderErr := func(msg string) {
		me.render(w, "register", registerData{baseData: newBase(r), Error: msg})
	}

	if email == "" || name == "" || password == "" {
		renderErr("All fields are required.")
		return
	}
	if password != confirm {
		renderErr("Passwords do not match.")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		renderErr("Internal error. Please try again.")
		return
	}

	isFirst := me.DB.CountUsers() == 0

	var token string
	emailVerified := isFirst || me.DisableEmailVerification || (me.Email != nil && me.Email.smtpHost == "")
	if !emailVerified {
		token, err = GenerateToken()
		if err != nil {
			renderErr("Internal error. Please try again.")
			return
		}
	}

	user, err := me.DB.CreateUser(email, name, string(hash), token, emailVerified, isFirst)
	if err != nil {
		renderErr("An account with that email already exists.")
		return
	}

	// Apply any pending invitation for this email.
	if inv, ok := me.DB.GetInvitationByEmail(email); ok {
		me.DB.AssignRole(user.ID, inv.TournamentID, inv.Role, inv.TeamID)
		me.DB.DeleteInvitation(inv.ID)
	}

	if emailVerified {
		// First user or email verification disabled — log them in directly.
		me.startSession(w, r, user.ID)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if me.Email != nil {
		go func() {
			if err := me.Email.SendVerificationEmail(email, name, token); err != nil {
				slog.Error("verification email failed", "err", err)
			}
		}()
	}
	me.render(w, "verify", verifyData{baseData: newBase(r)})
}

func (me *Env) VerifyEmail(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	token := r.URL.Query().Get("token")
	if token == "" {
		me.render(w, "verify", verifyData{baseData: newBase(r), Error: "Missing token."})
		return
	}
	user, err := me.DB.GetUserByVerificationToken(token)
	if err == sql.ErrNoRows || user.ID == 0 {
		me.render(w, "verify", verifyData{baseData: newBase(r), Error: "Invalid or already-used verification link."})
		return
	}
	me.DB.MarkEmailVerified(user.ID)
	me.render(w, "verify", verifyData{baseData: newBase(r), Success: true})
}

func (me *Env) ResendVerification(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	email := r.FormValue("email")
	user, err := me.DB.GetUserByEmail(email)
	if err != nil || user.ID == 0 || user.EmailVerified {
		// Don't leak whether email exists; just redirect.
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	token, err := GenerateToken()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	me.DB.SetVerificationToken(user.ID, token)
	if me.Email != nil {
		go func() {
			me.Email.SendVerificationEmail(user.Email, user.Name, token)
		}()
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (me *Env) LoginForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "login", loginData{baseData: newBase(r)})
}

func (me *Env) Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	if me.loginBlocked(ip) {
		http.Error(w, "Too many failed attempts — try again later", http.StatusTooManyRequests)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := me.DB.GetUserByEmail(email)
	if err != nil || user.ID == 0 {
		me.loginFailed(ip)
		me.render(w, "login", loginData{baseData: newBase(r), Error: "Invalid email or password."})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		me.loginFailed(ip)
		slog.Warn("login failed", "ip", ip, "email", email)
		me.render(w, "login", loginData{baseData: newBase(r), Error: "Invalid email or password."})
		return
	}

	if !user.EmailVerified {
		me.render(w, "login", loginData{
			baseData: newBase(r),
			Error:    "Please verify your email before logging in.",
		})
		return
	}

	me.loginSucceeded(ip)
	me.startSession(w, r, user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (me *Env) Logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	session, err := sessions.Start(w, r, false)
	if err == nil && session != nil {
		session.Destroy(w, r)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (me *Env) PasswordResetForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "passwordReset", passwordResetData{baseData: newBase(r)})
}

func (me *Env) PasswordReset(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	email := r.FormValue("email")
	user, err := me.DB.GetUserByEmail(email)
	if err == nil && user.ID > 0 {
		token, _ := GenerateToken()
		me.DB.SetResetToken(user.ID, token, time.Now().Add(time.Hour))
		if me.Email != nil {
			go func() {
				me.Email.SendPasswordResetEmail(user.Email, user.Name, token)
			}()
		}
	}
	// Always show success to avoid leaking whether email exists.
	me.render(w, "passwordReset", passwordResetData{baseData: newBase(r), Success: true})
}

func (me *Env) PasswordResetConfirmForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	token := r.URL.Query().Get("token")
	me.render(w, "passwordResetConfirm", passwordResetConfirmData{baseData: newBase(r), Token: token})
}

func (me *Env) PasswordResetConfirm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	token := r.FormValue("token")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	renderErr := func(msg string) {
		me.render(w, "passwordResetConfirm", passwordResetConfirmData{baseData: newBase(r), Token: token, Error: msg})
	}

	if password != confirm {
		renderErr("Passwords do not match.")
		return
	}
	if len(password) < 8 {
		renderErr("Password must be at least 8 characters.")
		return
	}

	user, err := me.DB.GetUserByResetToken(token)
	if err != nil || user.ID == 0 || time.Now().After(user.ResetExpires) {
		renderErr("Invalid or expired reset link.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		renderErr("Internal error.")
		return
	}
	me.DB.UpdatePassword(user.ID, string(hash))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (me *Env) startSession(w http.ResponseWriter, r *http.Request, userID int) {
	session, err := sessions.Start(w, r, true)
	if err != nil {
		slog.Error("session start failed", "err", err)
		return
	}
	session.Set("user_id", userID)
}
