package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGrowthWorkflows(t *testing.T) {
	dsn := os.Getenv("LINKPAY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set LINKPAY_TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "linkpay_growth_" + randomHex(8)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	t.Setenv("PUBLIC_TOKEN_SECRET", "growth-test-secret-with-at-least-thirty-two-characters")
	a, err := New(ctx, u.String(), "http://linkpay.test")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "[0-9]*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		sql, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = a.DB.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	server := httptest.NewServer(a.Routes(""))
	defer server.Close()
	request := func(method, path string, body any, cookie *http.Cookie) (int, map[string]any, *http.Cookie) {
		t.Helper()
		var data bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&data).Encode(body); err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, server.URL+path, &data)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", a.PublicURL)
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		out := map[string]any{}
		if res.StatusCode != 204 {
			if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
		}
		var session *http.Cookie
		for _, c := range res.Cookies() {
			if c.Name == "linkpay_session" {
				session = c
			}
		}
		return res.StatusCode, out, session
	}
	register := func(slug string) *http.Cookie {
		status, _, cookie := request("POST", "/api/v1/auth/register", map[string]any{"email": slug + "@example.test", "password": "A-long-test-password1!", "business_name": slug, "slug": slug}, nil)
		if status != 201 || cookie == nil {
			t.Fatalf("register %s: %d", slug, status)
		}
		return cookie
	}
	owner := register("growth-owner")
	other := register("growth-other")
	expectCounts := func(cookie *http.Cookie, wantActivity, wantEnquiries, wantQuotes float64) {
		t.Helper()
		status, counts, _ := request("GET", "/api/v1/notifications/counts", nil, cookie)
		if status != 200 || counts["activity"] != wantActivity || counts["enquiries"] != wantEnquiries || counts["quotes"] != wantQuotes {
			t.Fatalf("notification counts: %d %v; want activity=%v enquiries=%v quotes=%v", status, counts, wantActivity, wantEnquiries, wantQuotes)
		}
	}
	status, _, _ := request("GET", "/api/v1/notifications/counts", nil, nil)
	if status != 401 {
		t.Fatalf("anonymous notification counts: %d", status)
	}
	store := map[string]any{"name": "Growth Owner", "description": "Services", "whatsapp": "+263771234567", "email": "growth-owner@example.test", "currency": "USD", "timezone": "UTC", "theme_color": "#4f46e5", "published": true, "payment_instructions": "Pay by bank transfer using the invoice number"}
	status, _, _ = request("PATCH", "/api/v1/storefront", store, owner)
	if status != 200 {
		t.Fatalf("publish: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/public/stores/growth-owner/enquiries", map[string]any{"name": "Client", "email": "client@example.test", "message": "Please quote for work"}, nil)
	if status != 201 {
		t.Fatalf("enquiry: %d", status)
	}
	expectCounts(owner, 1, 1, 0)
	expectCounts(other, 0, 0, 0)
	status, inbox, _ := request("GET", "/api/v1/enquiries", nil, owner)
	if status != 200 {
		t.Fatalf("inbox: %d", status)
	}
	enquiryID := inbox["items"].([]any)[0].(map[string]any)["id"].(string)
	status, _, _ = request("PATCH", "/api/v1/enquiries/"+enquiryID+"/stage", map[string]any{"stage": "contacted"}, other)
	if status != 404 {
		t.Fatalf("cross-tenant enquiry changed: %d", status)
	}
	status, _, _ = request("PATCH", "/api/v1/enquiries/"+enquiryID+"/stage", map[string]any{"stage": "contacted"}, owner)
	if status != 200 {
		t.Fatalf("update stage: %d", status)
	}
	expectCounts(owner, 1, 0, 0)
	expires := time.Now().UTC().AddDate(0, 0, 14).Format("2006-01-02")
	status, created, _ := request("POST", "/api/v1/quotes", map[string]any{"enquiry_id": enquiryID, "description": "Two consultation sessions", "amount_minor": 25000, "expires_at": expires}, owner)
	if status != 201 {
		t.Fatalf("create quote: %d %v", status, created)
	}
	quoteID := created["id"].(string)
	changedCurrency := map[string]any{}
	for key, value := range store {
		changedCurrency[key] = value
	}
	changedCurrency["currency"] = "EUR"
	status, _, _ = request("PATCH", "/api/v1/storefront", changedCurrency, owner)
	if status != 409 {
		t.Fatalf("currency changed after quote: %d", status)
	}
	quoteURL, err := url.Parse(created["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	token := quoteURL.Path[len("/quote/"):]
	status, quote, _ := request("GET", "/api/v1/public/quotes/"+token, nil, nil)
	if status != 200 || quote["quote"].(map[string]any)["amount_minor"] != float64(25000) {
		t.Fatalf("public quote: %d %v", status, quote)
	}
	status, _, _ = request("POST", "/api/v1/public/quotes/"+token+"/respond", map[string]any{"decision": "accepted"}, nil)
	if status != 200 {
		t.Fatalf("accept quote: %d", status)
	}
	expectCounts(owner, 2, 0, 1)
	expectCounts(other, 0, 0, 0)
	status, _, _ = request("POST", "/api/v1/public/quotes/"+token+"/respond", map[string]any{"decision": "declined"}, nil)
	if status != 409 {
		t.Fatalf("accepted quote reversed: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/quotes/"+quoteID+"/convert", nil, other)
	if status != 404 {
		t.Fatalf("cross-tenant quote converted: %d", status)
	}
	status, converted, _ := request("POST", "/api/v1/quotes/"+quoteID+"/convert", nil, owner)
	if status != 201 {
		t.Fatalf("convert quote: %d %v", status, converted)
	}
	status, convertedDetail, _ := request("GET", "/api/v1/invoices/"+converted["id"].(string), nil, owner)
	if status != 200 || convertedDetail["customer_name"] != "Client" || convertedDetail["customer_email"] != "client@example.test" || convertedDetail["sender_business_name"] != "Growth Owner" || convertedDetail["sender_phone"] != "+263771234567" {
		t.Fatalf("converted quote invoice parties: %d %v", status, convertedDetail)
	}
	expectCounts(owner, 2, 0, 0)
	status, activity, _ := request("GET", "/api/v1/notifications", nil, owner)
	if status != 200 || len(activity["items"].([]any)) != 2 {
		t.Fatalf("owner activity: %d %v", status, activity)
	}
	notificationID := activity["items"].([]any)[0].(map[string]any)["id"].(string)
	status, _, _ = request("POST", "/api/v1/notifications/"+notificationID+"/read", nil, owner)
	if status != 204 {
		t.Fatalf("mark notification read: %d", status)
	}
	expectCounts(owner, 1, 0, 0)
	invoiceID := converted["id"].(string)
	status, _, _ = request("POST", "/api/v1/quotes/"+quoteID+"/convert", nil, owner)
	if status != 409 {
		t.Fatalf("quote converted twice: %d", status)
	}
	status, issued, _ := request("POST", "/api/v1/invoices/"+invoiceID+"/send", nil, owner)
	if status != 200 {
		t.Fatalf("issue invoice: %d", status)
	}
	invoiceURL, err := url.Parse(issued["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	invoiceToken := invoiceURL.Path[len("/invoice/"):]
	status, publicInvoice, _ := request("GET", "/api/v1/public/invoices/"+invoiceToken, nil, nil)
	if status != 200 || publicInvoice["business"].(map[string]any)["payment_instructions"] != store["payment_instructions"] {
		t.Fatalf("payment instructions: %d %v", status, publicInvoice)
	}
	status, _, _ = request("POST", "/api/v1/public/invoices/"+invoiceToken+"/payment-claims", map[string]any{"amount_minor": 25000, "reference": "TX-123"}, nil)
	if status != 201 {
		t.Fatalf("payment report: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/public/invoices/"+invoiceToken+"/payment-claims", map[string]any{"amount_minor": 25000, "reference": "TX-123"}, nil)
	if status != 409 {
		t.Fatalf("duplicate payment report: %d", status)
	}
	status, _, _ = request("GET", "/api/v1/invoices/"+invoiceID+"/payment-claims", nil, other)
	if status != 404 {
		t.Fatalf("cross-tenant payment report exposed: %d", status)
	}
	var claimCount int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM payment_claims WHERE invoice_id=$1 AND status='pending'`, invoiceID).Scan(&claimCount); err != nil {
		t.Fatal(err)
	}
	if claimCount != 1 {
		t.Fatalf("expected one unverified payment report, got %d", claimCount)
	}
	var claimID string
	if err := a.DB.QueryRow(ctx, `SELECT id::text FROM payment_claims WHERE invoice_id=$1 AND status='pending'`, invoiceID).Scan(&claimID); err != nil {
		t.Fatal(err)
	}
	status, _, _ = request("POST", "/api/v1/invoices/"+invoiceID+"/payments", map[string]any{"amount_minor": 25000, "method": "Bank transfer", "reference": "wrong", "claim_id": claimID, "idempotency_key": "growth-payment-claim-one"}, owner)
	if status != 409 {
		t.Fatalf("mismatched payment report verified: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/invoices/"+invoiceID+"/payments", map[string]any{"amount_minor": 25000, "method": "Bank transfer", "reference": "TX-123", "claim_id": claimID, "idempotency_key": "growth-payment-claim-one"}, owner)
	if status != 201 {
		t.Fatalf("verified payment report: %d", status)
	}
	var claimStatus string
	if err := a.DB.QueryRow(ctx, `SELECT status FROM payment_claims WHERE id=$1`, claimID).Scan(&claimStatus); err != nil {
		t.Fatal(err)
	}
	if claimStatus != "verified" {
		t.Fatalf("payment report status: %s", claimStatus)
	}
	status, _, _ = request("POST", "/api/v1/invoices/"+invoiceID+"/duplicate", map[string]any{"due_at": time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02")}, owner)
	if status != 201 {
		t.Fatalf("duplicate invoice: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/invoices/"+invoiceID+"/schedule", map[string]any{"interval_months": 1, "next_run_on": time.Now().UTC().Format("2006-01-02"), "active": true}, owner)
	if status != 200 {
		t.Fatalf("schedule invoice: %d", status)
	}
	if err := a.RunRecurringDrafts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.RunRecurringDrafts(ctx); err != nil {
		t.Fatal(err)
	}
	var invoiceCount int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM invoices WHERE business_id=(SELECT id FROM businesses WHERE slug='growth-owner')`).Scan(&invoiceCount); err != nil {
		t.Fatal(err)
	}
	if invoiceCount != 3 {
		t.Fatalf("expected quote, duplicate, and recurring invoices; got %d", invoiceCount)
	}
	if _, err := a.DB.Exec(ctx, `UPDATE invoice_schedules SET next_run_on=(current_date-interval '2 months')::date WHERE source_invoice_id=$1`, invoiceID); err != nil {
		t.Fatal(err)
	}
	if err := a.RunRecurringDrafts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.RunRecurringDrafts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM invoices WHERE business_id=(SELECT id FROM businesses WHERE slug='growth-owner')`).Scan(&invoiceCount); err != nil {
		t.Fatal(err)
	}
	if invoiceCount != 4 {
		t.Fatalf("stale schedule generated more than one catch-up draft: %d", invoiceCount)
	}
	var customerID string
	if err := a.DB.QueryRow(ctx, `SELECT id::text FROM customers WHERE business_id=(SELECT id FROM businesses WHERE slug='growth-owner') LIMIT 1`).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	status, _, _ = request("POST", "/api/v1/followups", map[string]any{"customer_id": customerID, "due_on": expires, "note": "Check project progress"}, other)
	if status != 404 {
		t.Fatalf("cross-tenant follow-up created: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/followups", map[string]any{"customer_id": customerID, "due_on": expires, "note": "Check project progress"}, owner)
	if status != 201 {
		t.Fatalf("follow-up: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/followups", map[string]any{"customer_id": customerID, "due_on": time.Now().UTC().Format("2006-01-02"), "note": "Call about next project"}, owner)
	if status != 201 {
		t.Fatalf("due follow-up: %d", status)
	}
	if err := a.RunFollowupAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	if err := a.RunFollowupAlerts(ctx); err != nil {
		t.Fatal(err)
	}
	var dueAlerts int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM merchant_notifications WHERE kind='followup'`).Scan(&dueAlerts); err != nil {
		t.Fatal(err)
	}
	if dueAlerts != 1 {
		t.Fatalf("expected one due follow-up alert, got %d", dueAlerts)
	}
	t.Setenv("SMTP_HOST", "")
	if err := a.RunNotifications(ctx); err != nil {
		t.Fatalf("merchant alert queue: %v", err)
	}
	var attempted int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM merchant_notifications WHERE attempts=1 AND email_status='pending'`).Scan(&attempted); err != nil {
		t.Fatal(err)
	}
	if attempted < 1 {
		t.Fatal("new activity did not reach the merchant alert queue")
	}
}
