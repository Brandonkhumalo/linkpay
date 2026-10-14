package platform

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	DB                *pgxpool.Pool
	PublicURL         string
	PublicTokenSecret string
	limiter           *limiter
}

type principal struct{ UserID, BusinessID, Role string }
type contextKey struct{}
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func validID(id string) bool { return uuidPattern.MatchString(id) }

func New(ctx context.Context, databaseURL, publicURL string) (*App, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	secret := os.Getenv("PUBLIC_TOKEN_SECRET")
	if len(secret) < 32 {
		db.Close()
		return nil, errors.New("PUBLIC_TOKEN_SECRET must have at least 32 characters")
	}
	return &App{DB: db, PublicURL: strings.TrimRight(publicURL, "/"), PublicTokenSecret: secret, limiter: &limiter{hits: make(map[string][]time.Time)}}, nil
}

func (a *App) Close() { a.DB.Close() }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func (a *App) invoiceToken(id string) string {
	m := hmac.New(sha256.New, []byte(a.PublicTokenSecret))
	_, _ = m.Write([]byte(id))
	return hex.EncodeToString(m.Sum(nil))
}
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		fail(w, 400, "Invalid request body")
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		fail(w, 400, "Only one JSON object is allowed")
		return false
	}
	return true
}
func (a *App) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/billing/paynow/result" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, err := url.Parse(origin)
				expected := a.PublicURL
				if expected == "" {
					scheme := "http"
					if r.TLS != nil {
						scheme = "https"
					}
					expected = scheme + "://" + r.Host
				}
				allowed, parseErr := url.Parse(expected)
				if err != nil || parseErr != nil || !strings.EqualFold(u.Host, allowed.Host) || u.Scheme != allowed.Scheme {
					fail(w, 403, "Invalid request origin")
					return
				}
			} else if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				fail(w, 403, "Invalid request origin")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) limit(key string, max int, period time.Duration) bool {
	a.limiter.mu.Lock()
	defer a.limiter.mu.Unlock()
	now := time.Now()
	if len(a.limiter.hits) >= 10000 {
		for k, v := range a.limiter.hits {
			if len(v) == 0 || now.Sub(v[len(v)-1]) >= 24*time.Hour {
				delete(a.limiter.hits, k)
			}
		}
		if len(a.limiter.hits) >= 10000 {
			if _, known := a.limiter.hits[key]; !known {
				return false
			}
		}
	}
	old := a.limiter.hits[key]
	kept := old[:0]
	for _, t := range old {
		if now.Sub(t) < period {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		a.limiter.hits[key] = kept
		return false
	}
	a.limiter.hits[key] = append(kept, now)
	return true
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func (a *App) rateLimit(prefix string, max int, period time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.limit(prefix+clientIP(r), max, period) {
			fail(w, 429, "Too many requests")
			return
		}
		next(w, r)
	}
}
func (a *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("linkpay_session")
		if err != nil {
			fail(w, 401, "Authentication required")
			return
		}
		var p principal
		err = a.DB.QueryRow(r.Context(), `SELECT s.user_id::text, COALESCE(m.business_id::text,''), COALESCE(m.role,'')
		FROM sessions s LEFT JOIN LATERAL (SELECT business_id,role FROM business_members WHERE user_id=s.user_id ORDER BY business_id LIMIT 1) m ON true
		WHERE s.token_hash=$1 AND s.expires_at>now()`, hash(cookie.Value)).Scan(&p.UserID, &p.BusinessID, &p.Role)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 401, "Session expired")
			return
		}
		if err != nil {
			unavailable(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, p)))
	})
}
func who(r *http.Request) principal { return r.Context().Value(contextKey{}).(principal) }
func (a *App) requireBusiness(next http.Handler) http.Handler {
	return a.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if who(r).BusinessID == "" {
			fail(w, 409, "Create a business first")
			return
		}
		next.ServeHTTP(w, r)
	}))
}
func (a *App) Routes(webDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
			next.ServeHTTP(w, req)
		})
	})
	r.Use(a.csrf)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.Ping(r.Context()); err != nil {
			fail(w, 503, "Database unavailable")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", a.rateLimit("register:", 5, time.Hour, a.register))
		r.Post("/auth/login", a.rateLimit("login:", 10, 15*time.Minute, a.login))
		r.Post("/auth/password-reset/request", a.rateLimit("reset-request:", 5, time.Hour, a.requestPasswordReset))
		r.Post("/auth/password-reset/confirm", a.rateLimit("reset-confirm:", 10, time.Hour, a.confirmPasswordReset))
		r.With(a.auth).Post("/auth/logout", a.logout)
		r.With(a.auth).Get("/me", a.me)
		r.With(a.auth).Get("/admin/overview", a.adminOverview)
		r.Get("/public/stores", a.publicStores)
		r.Get("/public/stores/{slug}", a.rateLimit("store:", 120, time.Minute, a.publicStore))
		r.Get("/public/stores/{slug}/logo", a.publicBusinessLogo)
		r.Get("/public/products/{id}/image", a.publicProductImage)
		r.Post("/public/stores/{slug}/enquiries", a.rateLimit("enquiry:", 10, time.Hour, a.publicEnquiry))
		r.Get("/public/invoices/{token}", a.rateLimit("invoice:", 120, time.Minute, a.publicInvoice))
		r.Get("/public/invoices/{token}/logo", a.publicInvoiceLogo)
		r.Post("/public/invoices/{token}/payment-claims", a.rateLimit("payment-claim:", 5, time.Hour, a.publicPaymentClaim))
		r.Get("/public/quotes/{token}", a.rateLimit("quote:", 120, time.Minute, a.publicQuote))
		r.Post("/public/quotes/{token}/respond", a.rateLimit("quote-response:", 10, time.Hour, a.respondQuote))
		r.Post("/billing/paynow/result", a.paynowResult)
		r.Group(func(r chi.Router) {
			r.Use(a.requireBusiness)
			r.Get("/businesses/current", a.business)
			r.Patch("/businesses/current", a.updateBusiness)
			r.Get("/storefront", a.business)
			r.Patch("/storefront", a.updateBusiness)
			r.Post("/storefront/logo", a.uploadBusinessLogo)
			r.Get("/storefront/logo", a.privateBusinessLogo)
			r.Delete("/storefront/logo", a.deleteBusinessLogo)
			r.Get("/products", a.products)
			r.Post("/products", a.createProduct)
			r.Patch("/products/{id}", a.updateProduct)
			r.Delete("/products/{id}", a.deleteProduct)
			r.Post("/products/{id}/image", a.uploadProductImage)
			r.Get("/products/{id}/image", a.privateProductImage)
			r.Get("/customers", a.customers)
			r.Post("/customers", a.createCustomer)
			r.Get("/enquiries", a.enquiries)
			r.Get("/enquiries/{id}", a.enquiryDetail)
			r.Patch("/enquiries/{id}/stage", a.updateEnquiryStage)
			r.Get("/quotes", a.quotes)
			r.Post("/quotes", a.createQuote)
			r.Post("/quotes/{id}/convert", a.convertQuote)
			r.Get("/notifications", a.notifications)
			r.Get("/notifications/counts", a.notificationCounts)
			r.Post("/notifications/{id}/read", a.readNotification)
			r.Get("/followups", a.followups)
			r.Post("/followups", a.createFollowup)
			r.Post("/followups/{id}/complete", a.completeFollowup)
			r.Get("/invoices", a.invoices)
			r.Post("/invoices", a.createInvoice)
			r.Get("/invoices/{id}", a.invoice)
			r.Get("/invoices/{id}/share-link", a.invoiceShareLink)
			r.Post("/invoices/{id}/send", a.sendInvoice)
			r.Post("/invoices/{id}/void", a.voidInvoice)
			r.Post("/invoices/{id}/payments", a.recordPayment)
			r.Get("/invoices/{id}/payment-claims", a.paymentClaims)
			r.Post("/invoices/{id}/payment-claims/{claimID}/dismiss", a.dismissPaymentClaim)
			r.Post("/invoices/{id}/duplicate", a.duplicateInvoice)
			r.Post("/invoices/{id}/schedule", a.updateInvoiceSchedule)
			r.Get("/invoices/{id}/schedule", a.invoiceSchedule)
			r.Get("/invoice-schedules", a.invoiceSchedules)
			r.Get("/invoices/{id}/pdf", a.invoicePDF)
			r.Get("/invoices/{id}/logo", a.privateInvoiceLogo)
			r.Get("/reminder-rules", a.reminderRules)
			r.Put("/reminder-rules", a.updateReminderRules)
			r.Get("/reminder-deliveries", a.reminderDeliveries)
			r.Get("/analytics/overview", a.overview)
			r.Get("/subscription", a.subscription)
			r.Post("/billing/checkout", a.rateLimit("billing:", 12, time.Hour, a.checkout))
			r.Get("/billing/payments", a.billingPayments)
			r.Post("/billing/payments/{id}/refresh", a.rateLimit("billing-refresh:", 30, time.Hour, a.refreshBillingPayment))
		})
	})
	if webDir != "" {
		r.Get("/app", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/login", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/register", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/billing/return", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/forgot-password", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/reset-password/{token}", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/admin", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/store/{slug}", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Get("/invoice/{token}", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, webDir+"/index.html") })
		r.Handle("/*", http.FileServer(http.Dir(webDir)))
	}
	return r
}

func logError(r *http.Request, err error) {
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("request failed", "path", r.URL.Path, "error", err)
	}
}
func unavailable(w http.ResponseWriter, r *http.Request, err error) {
	logError(r, err)
	fail(w, 500, "Request could not be completed")
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
