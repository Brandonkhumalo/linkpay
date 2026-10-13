package platform

import (
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)
var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(randomHex(16)), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}()

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type registerInput struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	BusinessName string `json:"business_name"`
	Slug         string `json:"slug"`
}

func validEmail(email string) bool {
	if len(email) > 254 {
		return false
	}
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email && !strings.ContainsAny(email, "<>\n\r")
}
func (a *App) setSession(w http.ResponseWriter, r *http.Request, userID string) error {
	token := randomHex(32)
	_, err := a.DB.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '30 days')`, hash(token), userID)
	if err != nil {
		return err
	}
	a.sessionCookie(w, token)
	return nil
}
func (a *App) sessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: "linkpay_session", Value: token, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600})
}
func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var in registerInput
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	if !validEmail(in.Email) || len(in.Password) < 12 || len(in.Password) > 72 || len(in.BusinessName) < 2 || len(in.BusinessName) > 120 || !slugPattern.MatchString(in.Slug) {
		fail(w, 400, "Provide a valid email, password of 12–72 characters, business name, and slug")
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	userID, businessID := newID(), newID()
	if _, err = tx.Exec(r.Context(), `INSERT INTO users(id,email,password_hash) VALUES($1,$2,$3)`, userID, in.Email, string(passwordHash)); err != nil {
		if isUniqueViolation(err) {
			fail(w, 409, "Email or storefront link is already in use")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO businesses(id,name,slug,email) VALUES($1,$2,$3,$4)`, businessID, in.BusinessName, in.Slug, in.Email); err != nil {
		if isUniqueViolation(err) {
			fail(w, 409, "Email or storefront link is already in use")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO business_members(business_id,user_id,role) VALUES($1,$2,'owner')`, businessID, userID); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO reminder_rules(business_id) VALUES($1)`, businessID); err != nil {
		unavailable(w, r, err)
		return
	}
	token := randomHex(32)
	if _, err = tx.Exec(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '30 days')`, hash(token), userID); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	a.sessionCookie(w, token)
	writeJSON(w, 201, map[string]string{"business_id": businessID, "slug": in.Slug})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	var id, passwordHash string
	err := a.DB.QueryRow(r.Context(), `SELECT id::text,password_hash FROM users WHERE email=$1`, in.Email).Scan(&id, &passwordHash)
	if err != nil && !isMissing(err) {
		unavailable(w, r, err)
		return
	}
	if isMissing(err) {
		passwordHash = string(dummyPasswordHash)
	}
	validPassword := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(in.Password)) == nil
	if err != nil || !validPassword {
		fail(w, 401, "Invalid credentials")
		return
	}
	if err = a.setSession(w, r, id); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"authenticated": true})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("linkpay_session"); err == nil {
		if _, err = a.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash=$1`, hash(cookie.Value)); err != nil {
			unavailable(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "linkpay_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0)})
	w.WriteHeader(204)
}
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	var email string
	err := a.DB.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, p.UserID).Scan(&email)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"id": p.UserID, "email": email, "business_id": p.BusinessID, "role": p.Role})
}
func isMissing(err error) bool { return err == pgx.ErrNoRows }
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
