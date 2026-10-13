package platform

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type resetRequest struct {
	Email string `json:"email"`
}
type resetConfirm struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (a *App) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in resetRequest
	if !decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmail(in.Email) {
		fail(w, 400, "Invalid email")
		return
	}
	if !a.limit("reset-email:"+hash(in.Email), 3, 24*time.Hour) {
		fail(w, 429, "Too many requests")
		return
	}
	response := func() {
		writeJSON(w, 202, map[string]string{"message": "If an account exists, a reset link will be emailed."})
	}
	if os.Getenv("SMTP_HOST") == "" || os.Getenv("SMTP_FROM") == "" || os.Getenv("SMTP_USER") == "" || os.Getenv("SMTP_PASSWORD") == "" || a.PublicURL == "" {
		fail(w, 503, "Password recovery is temporarily unavailable")
		return
	}
	var id string
	err := a.DB.QueryRow(r.Context(), `SELECT id::text FROM users WHERE email=$1`, in.Email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		response()
		return
	}
	if err != nil {
		logError(r, err)
		response()
		return
	}
	token := randomHex(32)
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		logError(r, err)
		response()
		return
	}
	defer tx.Rollback(r.Context())
	_, err = tx.Exec(r.Context(), `DELETE FROM password_reset_tokens WHERE user_id=$1 OR expires_at<now()`, id)
	if err != nil {
		logError(r, err)
		response()
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO password_reset_tokens(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '30 minutes')`, hash(token), id)
	if err != nil {
		logError(r, err)
		response()
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		logError(r, err)
		response()
		return
	}
	link := a.PublicURL + "/reset-password/" + token
	if err = sendPlainEmail(in.Email, "Reset your LinkPay password", "Open this link to reset your password. It expires in 30 minutes.\r\n\r\n"+link+"\r\n\r\nIf you did not request this, ignore this email.\r\n"); err != nil {
		slog.Error("password reset email failed", "error", err)
	}
	response()
}
func (a *App) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in resetConfirm
	if !decode(w, r, &in) {
		return
	}
	if len(in.Token) != 64 || len(in.Password) < 12 || len(in.Password) > 72 {
		fail(w, 400, "Invalid or expired reset request")
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
	var userID string
	err = tx.QueryRow(r.Context(), `SELECT user_id::text FROM password_reset_tokens WHERE token_hash=$1 AND expires_at>now() AND consumed_at IS NULL FOR UPDATE`, hash(in.Token)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 400, "Invalid or expired reset request")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE users SET password_hash=$1 WHERE id=$2`, string(passwordHash), userID); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM sessions WHERE user_id=$1`, userID); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE password_reset_tokens SET consumed_at=now() WHERE user_id=$1 AND consumed_at IS NULL`, userID); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO user_security_events(id,user_id,action) VALUES($1,$2,'password.reset')`, newID(), userID); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Password updated. Sign in again."})
}
