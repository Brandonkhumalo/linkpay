package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func TestTenantIsolationAndPaymentBoundary(t *testing.T) {
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
	schema := "linkpay_test_" + randomHex(8)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	t.Setenv("PUBLIC_TOKEN_SECRET", "test-secret-with-at-least-thirty-two-characters")
	a, err := New(ctx, u.String(), "http://linkpay.test")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	migrations, err := filepath.Glob(filepath.Join("..", "..", "migrations", "[0-9]*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(migrations)
	for _, path := range migrations {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := a.DB.Exec(ctx, string(migration), pgx.QueryExecModeSimpleProtocol); execErr != nil {
			t.Fatalf("%s: %v", path, execErr)
		}
	}
	server := httptest.NewServer(a.Routes(""))
	defer server.Close()
	request := func(method, path string, body any, cookie *http.Cookie) (int, map[string]any, *http.Cookie) {
		t.Helper()
		var buf bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&buf).Encode(body); err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, server.URL+path, &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "http://linkpay.test")
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var data map[string]any
		if res.StatusCode != 204 {
			if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
				t.Fatal(err)
			}
		}
		var session *http.Cookie
		for _, c := range res.Cookies() {
			if c.Name == "linkpay_session" {
				session = c
			}
		}
		return res.StatusCode, data, session
	}
	register := func(email, slug string) *http.Cookie {
		status, _, cookie := request("POST", "/api/v1/auth/register", map[string]any{"email": email, "password": "a-long-test-password", "business_name": slug, "slug": slug}, nil)
		if status != 201 || cookie == nil {
			t.Fatalf("register %s: %d", email, status)
		}
		return cookie
	}
	aCookie := register("tenant-a@example.test", "tenant-a")
	bCookie := register("tenant-b@example.test", "tenant-b")
	status, customer, _ := request("POST", "/api/v1/customers", map[string]any{"name": "Customer A", "email": "customer-a@example.test", "phone": "", "notes": ""}, aCookie)
	if status != 201 {
		t.Fatalf("create customer: %d", status)
	}
	input := map[string]any{"customer_id": customer["id"], "due_at": "2099-01-01", "discount_minor": 0, "tax_minor": 0, "notes": "", "items": []map[string]any{{"description": "Consulting", "quantity": 1, "unit_minor": 1000}}}
	status, _, _ = request("POST", "/api/v1/invoices", input, bCookie)
	if status != 400 {
		t.Fatalf("cross-tenant customer accepted: %d", status)
	}
	status, invoice, _ := request("POST", "/api/v1/invoices", input, aCookie)
	if status != 201 {
		t.Fatalf("create invoice: %d", status)
	}
	id := invoice["id"].(string)
	status, _, _ = request("GET", "/api/v1/invoices/"+id, nil, bCookie)
	if status != 404 {
		t.Fatalf("cross-tenant invoice exposed: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/invoices/"+id+"/send", nil, aCookie)
	if status != 200 {
		t.Fatalf("send invoice: %d", status)
	}
	payment := map[string]any{"amount_minor": 600, "method": "bank transfer", "reference": "T-1", "idempotency_key": "test-idempotency-key-1"}
	status, _, _ = request("POST", "/api/v1/invoices/"+id+"/payments", payment, bCookie)
	if status != 404 {
		t.Fatalf("cross-tenant payment accepted: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/invoices/"+id+"/payments", payment, aCookie)
	if status != 201 {
		t.Fatalf("payment failed: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/invoices/"+id+"/payments", payment, aCookie)
	if status != 200 {
		t.Fatalf("idempotent payment retry failed: %d", status)
	}
	payment["amount_minor"] = 500
	payment["idempotency_key"] = "test-idempotency-key-2"
	status, _, _ = request("POST", "/api/v1/invoices/"+id+"/payments", payment, aCookie)
	if status != 409 {
		t.Fatalf("overpayment accepted: %d", status)
	}
	var userID string
	if err = a.DB.QueryRow(ctx, `SELECT user_id::text FROM sessions WHERE token_hash=$1`, hash(aCookie.Value)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var businessID string
	if err = a.DB.QueryRow(ctx, `SELECT business_id::text FROM business_members WHERE user_id=$1`, userID).Scan(&businessID); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < listPageSize; n++ {
		if _, err = a.DB.Exec(ctx, `INSERT INTO customers(id,business_id,name,email,phone,notes) VALUES($1,$2,$3,'','','')`, newID(), businessID, "Paged customer "+strconv.Itoa(n)); err != nil {
			t.Fatal(err)
		}
	}
	status, firstPage, _ := request("GET", "/api/v1/customers?page=1", nil, aCookie)
	if status != 200 || len(firstPage["items"].([]any)) != listPageSize || firstPage["has_next"] != true || firstPage["has_prev"] != false {
		t.Fatalf("first customer page: %d %#v", status, firstPage)
	}
	status, secondPage, _ := request("GET", "/api/v1/customers?page=2", nil, aCookie)
	if status != 200 || len(secondPage["items"].([]any)) != 1 || secondPage["has_next"] != false || secondPage["has_prev"] != true {
		t.Fatalf("second customer page: %d %#v", status, secondPage)
	}
	status, otherTenantPage, _ := request("GET", "/api/v1/customers?page=1", nil, bCookie)
	if status != 200 || len(otherTenantPage["items"].([]any)) != 0 {
		t.Fatalf("cross-tenant customers exposed: %d %#v", status, otherTenantPage)
	}
	status, _, _ = request("GET", "/api/v1/customers?page=0", nil, aCookie)
	if status != 400 {
		t.Fatalf("invalid page accepted: %d", status)
	}
	billingID := newID()
	reference := "LP-" + strings.ReplaceAll(billingID, "-", "")
	pollURL := "https://www.paynow.co.zw/Interface/CheckPayment/?guid=test-guid"
	if _, err = a.DB.Exec(ctx, `INSERT INTO billing_payments(id,business_id,created_by,reference,plan,months,amount_minor,status,poll_url) VALUES($1,$2,$3,$4,'starter',1,300,'pending',$5)`, billingID, businessID, userID, reference, pollURL); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAYNOW_INTEGRATION_KEY", "test-paynow-integration-key")
	previousClient := paynowHTTP
	defer func() { paynowHTTP = previousClient }()
	providerStatus := "Paid"
	providerKey := os.Getenv("PAYNOW_INTEGRATION_KEY")
	paynowHTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != pollURL || r.Method != http.MethodPost {
			t.Errorf("unexpected poll request: %s %s", r.Method, r.URL)
		}
		fields := []paynowField{{"reference", reference}, {"paynowreference", "TEST-123"}, {"amount", "3.00"}, {"status", providerStatus}, {"pollurl", pollURL}}
		body := encodePaynow(fields, providerKey)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	providerKey = "incorrect-key"
	if err = a.verifyBillingPayment(ctx, billingID); err == nil {
		t.Fatal("invalid Paynow signature accepted")
	}
	providerKey = os.Getenv("PAYNOW_INTEGRATION_KEY")
	if err = a.verifyBillingPayment(ctx, billingID); err != nil {
		t.Fatalf("verified payment: %v", err)
	}
	var paidPlan string
	var paidUntil time.Time
	if err = a.DB.QueryRow(ctx, `SELECT plan,plan_expires_at FROM businesses WHERE id=$1`, businessID).Scan(&paidPlan, &paidUntil); err != nil {
		t.Fatal(err)
	}
	if paidPlan != "starter" || !paidUntil.After(time.Now()) {
		t.Fatal("verified payment did not grant Starter")
	}
	if err = a.verifyBillingPayment(ctx, billingID); err != nil {
		t.Fatalf("duplicate callback: %v", err)
	}
	var duplicateUntil time.Time
	if err = a.DB.QueryRow(ctx, `SELECT plan_expires_at FROM businesses WHERE id=$1`, businessID).Scan(&duplicateUntil); err != nil {
		t.Fatal(err)
	}
	if !duplicateUntil.Equal(paidUntil) {
		t.Fatal("duplicate callback extended the plan")
	}
	providerStatus = "Refunded"
	if err = a.verifyBillingPayment(ctx, billingID); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if err = a.DB.QueryRow(ctx, `SELECT plan FROM businesses WHERE id=$1`, businessID).Scan(&paidPlan); err != nil {
		t.Fatal(err)
	}
	if paidPlan != "free" {
		t.Fatal("refund did not revoke access")
	}
	resetToken := randomHex(32)
	if _, err = a.DB.Exec(ctx, `INSERT INTO password_reset_tokens(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '30 minutes')`, hash(resetToken), userID); err != nil {
		t.Fatal(err)
	}
	status, _, _ = request("POST", "/api/v1/auth/password-reset/confirm", map[string]any{"token": resetToken, "password": "a-new-long-test-password"}, nil)
	if status != 200 {
		t.Fatalf("password reset failed: %d", status)
	}
	status, _, _ = request("GET", "/api/v1/me", nil, aCookie)
	if status != 401 {
		t.Fatalf("old session survived password reset: %d", status)
	}
}
