package platform

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func totpCode(secret []byte, step int64) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(message[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	number := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", number%1000000)
}
func validTOTP(encoded, submitted string) bool {
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.TrimRight(strings.ToUpper(strings.TrimSpace(encoded)), "="))
	if err != nil || len(secret) < 20 || len(submitted) != 6 {
		return false
	}
	step := time.Now().Unix() / 30
	for delta := -1; delta <= 1; delta++ {
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, step+int64(delta))), []byte(submitted)) == 1 {
			return true
		}
	}
	return false
}
func (a *App) adminOverview(w http.ResponseWriter, r *http.Request) {
	if !a.limit("admin:"+clientIP(r), 5, time.Minute) {
		fail(w, 429, "Too many requests")
		return
	}
	adminEmail, secret := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_TOTP_SECRET")
	if adminEmail == "" || secret == "" {
		fail(w, 404, "Not found")
		return
	}
	var email string
	err := a.DB.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, who(r).UserID).Scan(&email)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if !strings.EqualFold(email, adminEmail) || !validTOTP(secret, r.Header.Get("X-Admin-Code")) {
		fail(w, 403, "Admin authentication required")
		return
	}
	var users, businesses, published, invoices, failedReminders int
	for _, query := range []struct {
		sql    string
		target *int
	}{
		{`SELECT count(*) FROM users`, &users}, {`SELECT count(*) FROM businesses`, &businesses}, {`SELECT count(*) FROM businesses WHERE published=true`, &published}, {`SELECT count(*) FROM invoices`, &invoices}, {`SELECT count(*) FROM reminder_deliveries WHERE status='failed'`, &failedReminders},
	} {
		if err = a.DB.QueryRow(r.Context(), query.sql).Scan(query.target); err != nil {
			unavailable(w, r, err)
			return
		}
	}
	_, err = a.DB.Exec(r.Context(), `INSERT INTO admin_audit_logs(id,actor_id,action) VALUES($1,$2,'overview.viewed')`, newID(), who(r).UserID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]int{"users": users, "businesses": businesses, "published_storefronts": published, "invoices": invoices, "failed_reminders": failedReminders})
}
