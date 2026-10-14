package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
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
	t.Setenv("MEDIA_DIR", t.TempDir())
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
	logoStatus := func(method, path string, cookie *http.Cookie, body io.Reader, contentType string) int {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", a.PublicURL)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	register := func(email, slug, trialPlan string) *http.Cookie {
		status, _, cookie := request("POST", "/api/v1/auth/register", map[string]any{"email": email, "password": "A-long-test-password1!", "business_name": slug, "slug": slug, "trial_plan": trialPlan}, nil)
		if status != 201 || cookie == nil {
			t.Fatalf("register %s: %d", email, status)
		}
		return cookie
	}
	status, _, _ := request("POST", "/api/v1/auth/register", map[string]any{"email": "invalid-trial@example.test", "password": "A-long-test-password1!", "business_name": "Invalid Trial", "slug": "invalid-trial", "trial_plan": "premium"}, nil)
	if status != 400 {
		t.Fatalf("invalid signup plan accepted: %d", status)
	}
	freeCookie := register("tenant-free@example.test", "tenant-free", "free")
	var freePlan string
	var freeHasTrial, freeHasExpiry bool
	if err = a.DB.QueryRow(ctx, `SELECT plan,trial_plan IS NOT NULL OR trial_started_at IS NOT NULL OR trial_expires_at IS NOT NULL,plan_expires_at IS NOT NULL FROM businesses WHERE slug='tenant-free'`).Scan(&freePlan, &freeHasTrial, &freeHasExpiry); err != nil {
		t.Fatal(err)
	}
	if freePlan != "free" || freeHasTrial || freeHasExpiry {
		t.Fatalf("free signup created a trial: plan=%s trial=%t expiry=%t", freePlan, freeHasTrial, freeHasExpiry)
	}
	status, freeSubscription, _ := request("GET", "/api/v1/subscription", nil, freeCookie)
	if status != 200 || freeSubscription["plan"] != "free" || freeSubscription["trial_active"] != false || freeSubscription["invoice_monthly_limit"] != float64(30) {
		t.Fatalf("free signup subscription: %d %v", status, freeSubscription)
	}
	status, _, _ = request("POST", "/api/v1/auth/register", map[string]any{"email": "tenant-default@example.test", "password": "A-long-test-password1!", "business_name": "Default Free", "slug": "tenant-default"}, nil)
	if status != 201 {
		t.Fatalf("signup without plan did not default to Free: %d", status)
	}
	aCookie := register("tenant-a@example.test", "tenant-a", "starter")
	bCookie := register("tenant-b@example.test", "tenant-b", "business")
	for _, trial := range []struct {
		slug, plan   string
		productLimit float64
		invoiceLimit float64
		cookie       *http.Cookie
	}{{"tenant-a", "starter", 30, 60, aCookie}, {"tenant-b", "business", 500, 500, bCookie}} {
		var storedPlan, trialPlan string
		var started, expires, planExpires time.Time
		var seconds float64
		if err = a.DB.QueryRow(ctx, `SELECT plan,trial_plan,trial_started_at,trial_expires_at,plan_expires_at,EXTRACT(EPOCH FROM (trial_expires_at-trial_started_at))::double precision FROM businesses WHERE slug=$1`, trial.slug).Scan(&storedPlan, &trialPlan, &started, &expires, &planExpires, &seconds); err != nil {
			t.Fatal(err)
		}
		if storedPlan != trial.plan || trialPlan != trial.plan || seconds != 168*60*60 || !expires.Equal(planExpires) {
			t.Fatalf("incorrect %s trial window: %s %s %v %v %v", trial.slug, storedPlan, trialPlan, started, expires, planExpires)
		}
		status, subscription, _ := request("GET", "/api/v1/subscription", nil, trial.cookie)
		if status != 200 || subscription["plan"] != trial.plan || subscription["trial_active"] != true || subscription["product_limit"] != trial.productLimit || subscription["invoice_monthly_limit"] != trial.invoiceLimit {
			t.Fatalf("%s subscription: %d %v", trial.slug, status, subscription)
		}
	}
	if _, err = a.DB.Exec(ctx, `UPDATE businesses SET trial_started_at=now()-interval '169 hours',trial_expires_at=now()-interval '1 hour',plan_expires_at=now()-interval '1 hour' WHERE slug='tenant-b'`); err != nil {
		t.Fatal(err)
	}
	status, expiredTrial, _ := request("GET", "/api/v1/subscription", nil, bCookie)
	if status != 200 || expiredTrial["plan"] != "free" || expiredTrial["trial_active"] != false || expiredTrial["product_limit"] != float64(5) {
		t.Fatalf("unpaid trial did not fall back to Free: %d %v", status, expiredTrial)
	}
	status, directory, _ := request("GET", "/api/v1/public/stores", nil, nil)
	if status != 200 || len(directory["items"].([]any)) != 0 {
		t.Fatalf("draft storefront visible in directory: %d %v", status, directory)
	}
	var logoBody bytes.Buffer
	logoForm := multipart.NewWriter(&logoBody)
	logoPart, err := logoForm.CreateFormFile("image", "logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(logoPart, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err = logoForm.Close(); err != nil {
		t.Fatal(err)
	}
	if status = logoStatus("POST", "/api/v1/storefront/logo", aCookie, &logoBody, logoForm.FormDataContentType()); status != 200 {
		t.Fatalf("upload logo: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/storefront/logo", aCookie, nil, ""); status != 200 {
		t.Fatalf("private logo: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/storefront/logo", bCookie, nil, ""); status != 404 {
		t.Fatalf("other tenant saw logo: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/public/stores/tenant-a/logo", nil, nil, ""); status != 404 {
		t.Fatalf("draft logo was public: %d", status)
	}
	storeUpdate := map[string]any{"name": "Tenant A", "description": "A shop", "whatsapp": "", "email": "tenant-a@example.test", "currency": "USD", "timezone": "UTC", "theme_color": "#4f46e5", "published": true}
	status, _, _ = request("PATCH", "/api/v1/storefront", storeUpdate, aCookie)
	if status != 400 {
		t.Fatalf("storefront published without WhatsApp: %d", status)
	}
	storeUpdate["whatsapp"] = "+263771234567"
	status, _, _ = request("PATCH", "/api/v1/storefront", storeUpdate, aCookie)
	if status != 200 {
		t.Fatalf("publish storefront: %d", status)
	}
	status, directory, _ = request("GET", "/api/v1/public/stores", nil, nil)
	shops := directory["items"].([]any)
	if status != 200 || len(shops) != 1 || shops[0].(map[string]any)["slug"] != "tenant-a" || shops[0].(map[string]any)["whatsapp"] != "+263771234567" || shops[0].(map[string]any)["has_logo"] != true || shops[0].(map[string]any)["logo_version"] == "" {
		t.Fatalf("public directory: %d %v", status, directory)
	}
	if status = logoStatus("GET", "/api/v1/public/stores/tenant-a/logo", nil, nil, ""); status != 200 {
		t.Fatalf("published logo: %d", status)
	}
	if status = logoStatus("DELETE", "/api/v1/storefront/logo", aCookie, nil, ""); status != 204 {
		t.Fatalf("remove logo: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/public/stores/tenant-a/logo", nil, nil, ""); status != 404 {
		t.Fatalf("removed logo was public: %d", status)
	}
	storeUpdate["profile_type"] = "creator"
	storeUpdate["creator_niche"] = "Food"
	storeUpdate["creator_location"] = "Harare"
	storeUpdate["whatsapp"] = ""
	storeUpdate["creator_portfolio_urls"] = []string{"javascript:alert(1)"}
	status, _, _ = request("PATCH", "/api/v1/storefront", storeUpdate, aCookie)
	if status != 400 {
		t.Fatalf("unsafe portfolio link accepted: %d", status)
	}
	storeUpdate["creator_portfolio_urls"] = []string{"https://example.com/work"}
	status, _, _ = request("PATCH", "/api/v1/storefront", storeUpdate, aCookie)
	if status != 200 {
		t.Fatalf("creator profile update: %d", status)
	}
	status, directory, _ = request("GET", "/api/v1/public/stores?type=creator", nil, nil)
	if status != 200 || len(directory["items"].([]any)) != 1 || directory["items"].([]any)[0].(map[string]any)["creator_niche"] != "Food" {
		t.Fatalf("creator directory: %d %v", status, directory)
	}
	status, _, _ = request("GET", "/api/v1/public/stores?type=unknown", nil, nil)
	if status != 400 {
		t.Fatalf("invalid directory filter accepted: %d", status)
	}
	status, packageView, _ := request("POST", "/api/v1/products", map[string]any{"name": "Sponsored video", "description": "Custom campaign", "kind": "service", "price_minor": 0, "price_on_request": true, "active": true}, aCookie)
	if status != 201 {
		t.Fatalf("creator package: %d %v", status, packageView)
	}
	status, _, _ = request("POST", "/api/v1/products", map[string]any{"name": "Invalid quote", "kind": "product", "price_minor": 0, "price_on_request": true, "active": true}, aCookie)
	if status != 400 {
		t.Fatalf("product quote accepted: %d", status)
	}
	status, creatorStore, _ := request("GET", "/api/v1/public/stores/tenant-a", nil, nil)
	if status != 200 || creatorStore["business"].(map[string]any)["profile_type"] != "creator" || creatorStore["products"].([]any)[0].(map[string]any)["price_on_request"] != true {
		t.Fatalf("public creator package: %d %v", status, creatorStore)
	}
	brief := map[string]any{"product_id": packageView["id"], "name": "Brand manager", "email": "brand@example.test", "message": "We would like to collaborate", "brand_name": "Example Brand", "deliverables": "Two videos", "budget_minor": 50000, "campaign_deadline": "2099-01-01", "usage_rights": "Organic social for six months"}
	status, _, _ = request("POST", "/api/v1/public/stores/tenant-a/enquiries", brief, nil)
	if status != 201 {
		t.Fatalf("brand brief: %d", status)
	}
	status, tenantBriefs, _ := request("GET", "/api/v1/enquiries", nil, aCookie)
	if status != 200 || len(tenantBriefs["items"].([]any)) != 1 || tenantBriefs["items"].([]any)[0].(map[string]any)["brand_name"] != "Example Brand" {
		t.Fatalf("creator briefs: %d %v", status, tenantBriefs)
	}
	briefID := tenantBriefs["items"].([]any)[0].(map[string]any)["id"].(string)
	status, detail, _ := request("GET", "/api/v1/enquiries/"+briefID, nil, aCookie)
	if status != 200 || detail["deliverables"] != "Two videos" {
		t.Fatalf("private brief detail: %d %v", status, detail)
	}
	status, _, _ = request("GET", "/api/v1/enquiries/"+briefID, nil, bCookie)
	if status != 404 {
		t.Fatalf("cross-tenant brief detail exposed: %d", status)
	}
	generalBrief := map[string]any{"name": "Agency contact", "email": "agency@example.test", "message": "We would like to collaborate", "brand_name": "Agency", "deliverables": "A campaign concept"}
	status, _, _ = request("POST", "/api/v1/public/stores/tenant-a/enquiries", generalBrief, nil)
	if status != 201 {
		t.Fatalf("general partnership brief: %d", status)
	}
	status, otherBriefs, _ := request("GET", "/api/v1/enquiries", nil, bCookie)
	if status != 200 || len(otherBriefs["items"].([]any)) != 0 {
		t.Fatalf("cross-tenant brief exposed: %d %v", status, otherBriefs)
	}
	delete(brief, "deliverables")
	status, _, _ = request("POST", "/api/v1/public/stores/tenant-a/enquiries", brief, nil)
	if status != 400 {
		t.Fatalf("incomplete brand brief accepted: %d", status)
	}
	for i := 0; i < listPageSize; i++ {
		name := "Directory Shop " + strconv.Itoa(i)
		if _, err = a.DB.Exec(ctx, `INSERT INTO businesses(id,name,slug,published) VALUES($1,$2,$3,true)`, newID(), name, "directory-shop-"+strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
	}
	status, directory, _ = request("GET", "/api/v1/public/stores?page=1", nil, nil)
	if status != 200 || len(directory["items"].([]any)) != listPageSize || directory["has_next"] != true || directory["has_prev"] != false {
		t.Fatalf("first directory page: %d %v", status, directory)
	}
	status, directory, _ = request("GET", "/api/v1/public/stores?page=2", nil, nil)
	if status != 200 || len(directory["items"].([]any)) != 1 || directory["has_next"] != false || directory["has_prev"] != true {
		t.Fatalf("second directory page: %d %v", status, directory)
	}
	status, customer, _ := request("POST", "/api/v1/customers", map[string]any{"name": "Customer A", "email": "customer-a@example.test", "phone": "", "notes": ""}, aCookie)
	if status != 201 {
		t.Fatalf("create customer: %d", status)
	}
	input := map[string]any{"customer_id": customer["id"], "recipient_business_name": "Recipient Ltd", "recipient_email": "", "recipient_phone": "", "sender_phone": "+263771234567", "due_at": "2099-01-01", "discount_minor": 0, "tax_minor": 0, "notes": "", "items": []map[string]any{{"description": "Consulting", "quantity": 1, "unit_minor": 1000}}}
	input["design_template"] = "modern"
	input["design_color"] = "#245b76"
	input["show_logo"] = true
	status, _, _ = request("POST", "/api/v1/invoices", input, aCookie)
	if status != 400 {
		t.Fatalf("invoice accepted missing logo: %d", status)
	}
	var invoiceLogoBody bytes.Buffer
	invoiceLogoForm := multipart.NewWriter(&invoiceLogoBody)
	invoiceLogoPart, err := invoiceLogoForm.CreateFormFile("image", "invoice-logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(invoiceLogoPart, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err = invoiceLogoForm.Close(); err != nil {
		t.Fatal(err)
	}
	if status = logoStatus("POST", "/api/v1/storefront/logo", aCookie, &invoiceLogoBody, invoiceLogoForm.FormDataContentType()); status != 200 {
		t.Fatalf("invoice logo upload: %d", status)
	}
	badDesign := map[string]any{}
	for key, value := range input {
		badDesign[key] = value
	}
	badDesign["design_template"] = "untrusted"
	status, _, _ = request("POST", "/api/v1/invoices", badDesign, aCookie)
	if status != 400 {
		t.Fatalf("invalid invoice template accepted: %d", status)
	}
	badDesign["design_template"] = "modern"
	badDesign["design_color"] = "url(javascript:alert(1))"
	status, _, _ = request("POST", "/api/v1/invoices", badDesign, aCookie)
	if status != 400 {
		t.Fatalf("invalid invoice color accepted: %d", status)
	}
	missingName := map[string]any{}
	for key, value := range input {
		missingName[key] = value
	}
	missingName["recipient_business_name"] = ""
	status, _, _ = request("POST", "/api/v1/invoices", missingName, aCookie)
	if status != 400 {
		t.Fatalf("invoice without recipient business name accepted: %d", status)
	}
	badPhone := map[string]any{}
	for key, value := range input {
		badPhone[key] = value
	}
	badPhone["sender_phone"] = "invalid"
	status, _, _ = request("POST", "/api/v1/invoices", badPhone, aCookie)
	if status != 400 {
		t.Fatalf("invoice without valid sender phone accepted: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/invoices", input, bCookie)
	if status != 400 {
		t.Fatalf("cross-tenant customer accepted: %d", status)
	}
	status, invoice, _ := request("POST", "/api/v1/invoices", input, aCookie)
	if status != 201 {
		t.Fatalf("create invoice: %d", status)
	}
	id := invoice["id"].(string)
	status, invoiceDetail, _ := request("GET", "/api/v1/invoices/"+id, nil, aCookie)
	if status != 200 || invoiceDetail["customer_name"] != "Recipient Ltd" || invoiceDetail["customer_email"] != "" || invoiceDetail["customer_phone"] != "" || invoiceDetail["sender_business_name"] != "Tenant A" || invoiceDetail["sender_phone"] != "+263771234567" || invoiceDetail["design_template"] != "modern" || invoiceDetail["design_color"] != "#245b76" || invoiceDetail["show_logo"] != true {
		t.Fatalf("invoice party details: %d %v", status, invoiceDetail)
	}
	if status = logoStatus("GET", "/api/v1/invoices/"+id+"/logo", aCookie, nil, ""); status != 200 {
		t.Fatalf("owner invoice logo: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/invoices/"+id+"/logo", bCookie, nil, ""); status != 404 {
		t.Fatalf("other tenant saw invoice logo: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/public/invoices/"+a.invoiceToken(id)+"/logo", nil, nil, ""); status != 404 {
		t.Fatalf("draft invoice logo was public: %d", status)
	}
	status, _, _ = request("GET", "/api/v1/invoices/"+id, nil, bCookie)
	if status != 404 {
		t.Fatalf("cross-tenant invoice exposed: %d", status)
	}
	status, issuedInvoice, _ := request("POST", "/api/v1/invoices/"+id+"/send", nil, aCookie)
	if status != 200 {
		t.Fatalf("send invoice: %d", status)
	}
	issuedURL, err := url.Parse(issuedInvoice["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	status, publicInvoice, _ := request("GET", "/api/v1/public/invoices/"+strings.TrimPrefix(issuedURL.Path, "/invoice/"), nil, nil)
	if status != 200 || publicInvoice["invoice"].(map[string]any)["customer_name"] != "Recipient Ltd" || publicInvoice["invoice"].(map[string]any)["customer_email"] != "" || publicInvoice["business"].(map[string]any)["name"] != "Tenant A" || publicInvoice["business"].(map[string]any)["phone"] != "+263771234567" {
		t.Fatalf("public invoice parties: %d %v", status, publicInvoice)
	}
	if status = logoStatus("GET", "/api/v1/public/invoices/"+a.invoiceToken(id)+"/logo", nil, nil, ""); status != 200 {
		t.Fatalf("public invoice logo: %d", status)
	}
	if status = logoStatus("DELETE", "/api/v1/storefront/logo", aCookie, nil, ""); status != 204 {
		t.Fatalf("remove storefront logo: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/public/invoices/"+a.invoiceToken(id)+"/logo", nil, nil, ""); status != 200 {
		t.Fatalf("invoice logo lost after storefront removal: %d", status)
	}
	if status = logoStatus("GET", "/api/v1/invoices/"+id+"/pdf", aCookie, nil, ""); status != 200 {
		t.Fatalf("invoice PDF with retained logo: %d", status)
	}
	input["show_logo"] = false
	if _, err = a.DB.Exec(ctx, `UPDATE customers SET name='Changed Recipient' WHERE id=$1`, customer["id"]); err != nil {
		t.Fatal(err)
	}
	if _, err = a.DB.Exec(ctx, `UPDATE businesses SET name='Changed Sender' WHERE slug='tenant-a'`); err != nil {
		t.Fatal(err)
	}
	status, invoiceDetail, _ = request("GET", "/api/v1/invoices/"+id, nil, aCookie)
	if status != 200 || invoiceDetail["customer_name"] != "Recipient Ltd" || invoiceDetail["sender_business_name"] != "Tenant A" {
		t.Fatalf("invoice parties changed with profiles: %d %v", status, invoiceDetail)
	}
	if _, err = a.DB.Exec(ctx, `UPDATE customers SET name='Customer A' WHERE id=$1`, customer["id"]); err != nil {
		t.Fatal(err)
	}
	if _, err = a.DB.Exec(ctx, `UPDATE businesses SET name='Tenant A' WHERE slug='tenant-a'`); err != nil {
		t.Fatal(err)
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
	status, _, _ = request("POST", "/api/v1/billing/checkout", map[string]any{"topup_invoices": 31}, aCookie)
	if status != 400 {
		t.Fatalf("invalid top-up quantity accepted: %d", status)
	}
	status, _, _ = request("POST", "/api/v1/billing/checkout", map[string]any{"plan": "starter", "months": 1, "topup_invoices": 30}, aCookie)
	if status != 400 {
		t.Fatalf("mixed plan and top-up accepted: %d", status)
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
	providerStatus, providerReference, providerAmount := "Paid", reference, "3.00"
	providerKey := os.Getenv("PAYNOW_INTEGRATION_KEY")
	paynowHTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == paynowEndpoint {
			body, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				return nil, readErr
			}
			fields, parseErr := parsePaynow(string(body), providerKey)
			if parseErr != nil || fields["amount"] != "1.00" || fields["additionalinfo"] != "LinkPay 30 invoice top-up" {
				t.Errorf("invalid top-up checkout request: %v %v", fields, parseErr)
			}
			response := encodePaynow([]paynowField{{"status", "Ok"}, {"browserurl", "https://www.paynow.co.zw/payment/test"}, {"pollurl", pollURL}}, providerKey)
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
		}
		if r.URL.String() != pollURL || r.Method != http.MethodPost {
			t.Errorf("unexpected poll request: %s %s", r.Method, r.URL)
		}
		fields := []paynowField{{"reference", providerReference}, {"paynowreference", "TEST-123"}, {"amount", providerAmount}, {"status", providerStatus}, {"pollurl", pollURL}}
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
	status, subscription, _ := request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["trial_active"] != false {
		t.Fatalf("paid subscription still presented as a trial: %d %v", status, subscription)
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
	t.Setenv("PAYNOW_INTEGRATION_ID", "1234")
	a.PublicURL = "https://linkpay.test"
	status, checkout, _ := request("POST", "/api/v1/billing/checkout", map[string]any{"topup_invoices": 30}, aCookie)
	if status != 201 || checkout["checkout_url"] != "https://www.paynow.co.zw/payment/test" {
		t.Fatalf("top-up checkout failed: %d %v", status, checkout)
	}
	var checkoutKind string
	var checkoutQuantity int
	var checkoutPrice int64
	if err = a.DB.QueryRow(ctx, `SELECT kind,invoice_quantity,amount_minor FROM billing_payments WHERE id=$1`, checkout["id"]).Scan(&checkoutKind, &checkoutQuantity, &checkoutPrice); err != nil || checkoutKind != "invoice_topup" || checkoutQuantity != 30 || checkoutPrice != 100 {
		t.Fatalf("top-up checkout row: %s %d %d %v", checkoutKind, checkoutQuantity, checkoutPrice, err)
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["invoice_topup_balance"] != float64(0) {
		t.Fatalf("unverified checkout granted invoices: %d %v", status, subscription)
	}
	a.PublicURL = "http://linkpay.test"
	for n := 0; n < 59; n++ {
		if _, err = a.DB.Exec(ctx, `INSERT INTO invoices(id,business_id,customer_id,number,currency,due_at,subtotal_minor,total_minor) VALUES($1,$2,$3,$4,'USD','2099-01-01',0,0)`, newID(), businessID, customer["id"], 1000+n); err != nil {
			t.Fatal(err)
		}
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["invoices_used"] != float64(60) || subscription["invoice_topup_balance"] != float64(0) {
		t.Fatalf("subscription quota usage: %d %v", status, subscription)
	}
	status, _, _ = request("POST", "/api/v1/invoices", input, aCookie)
	if status != 403 {
		t.Fatalf("invoice exceeded plan before top-up: %d", status)
	}
	topupID := newID()
	providerReference, providerAmount = "LP-"+strings.ReplaceAll(topupID, "-", ""), "1.00"
	if _, err = a.DB.Exec(ctx, `INSERT INTO billing_payments(id,business_id,created_by,reference,kind,invoice_quantity,amount_minor,status,poll_url) VALUES($1,$2,$3,$4,'invoice_topup',30,100,'pending',$5)`, topupID, businessID, userID, providerReference, pollURL); err != nil {
		t.Fatal(err)
	}
	if err = a.verifyBillingPayment(ctx, topupID); err != nil {
		t.Fatalf("verified top-up: %v", err)
	}
	if err = a.verifyBillingPayment(ctx, topupID); err != nil {
		t.Fatalf("duplicate top-up callback: %v", err)
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["invoice_topup_balance"] != float64(30) {
		t.Fatalf("top-up not credited once: %d %v", status, subscription)
	}
	input["recipient_email"] = "accounts@recipient.test"
	input["recipient_phone"] = "+263 77 123 4567"
	status, topupInvoice, _ := request("POST", "/api/v1/invoices", input, aCookie)
	if status != 201 {
		t.Fatalf("invoice with top-up: %d", status)
	}
	status, topupDetail, _ := request("GET", "/api/v1/invoices/"+topupInvoice["id"].(string), nil, aCookie)
	if status != 200 || topupDetail["customer_email"] != "accounts@recipient.test" || topupDetail["customer_phone"] != "+263 77 123 4567" {
		t.Fatalf("optional recipient contacts: %d %v", status, topupDetail)
	}
	var quotaSource string
	if err = a.DB.QueryRow(ctx, `SELECT quota_source FROM invoices WHERE id=$1`, topupInvoice["id"]).Scan(&quotaSource); err != nil || quotaSource != "invoice_topup" {
		t.Fatalf("invoice did not use purchased credit: %q %v", quotaSource, err)
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["invoices_used"] != float64(60) || subscription["invoice_topup_balance"] != float64(29) {
		t.Fatalf("top-up use changed monthly allowance: %d %v", status, subscription)
	}
	if _, err = a.DB.Exec(ctx, `UPDATE invoices SET created_at=date_trunc('month',now())-interval '1 day' WHERE business_id=$1 AND quota_source='subscription'`, businessID); err != nil {
		t.Fatal(err)
	}
	status, _, _ = request("POST", "/api/v1/invoices", input, aCookie)
	if status != 201 {
		t.Fatalf("new month did not restore plan invoices: %d", status)
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["invoices_used"] != float64(1) || subscription["invoice_topup_balance"] != float64(29) {
		t.Fatalf("new month spent purchased invoices: %d %v", status, subscription)
	}
	providerStatus = "Refunded"
	if err = a.verifyBillingPayment(ctx, topupID); err != nil {
		t.Fatalf("top-up refund: %v", err)
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["invoice_topup_balance"] != float64(0) || subscription["invoice_topup_debt"] != float64(1) {
		t.Fatalf("used refunded credit did not create debt: %d %v", status, subscription)
	}
	providerStatus, providerAmount = "Paid", "2.00"
	secondTopupID := newID()
	providerReference = "LP-" + strings.ReplaceAll(secondTopupID, "-", "")
	if _, err = a.DB.Exec(ctx, `INSERT INTO billing_payments(id,business_id,created_by,reference,kind,invoice_quantity,amount_minor,status,poll_url) VALUES($1,$2,$3,$4,'invoice_topup',100,200,'pending',$5)`, secondTopupID, businessID, userID, providerReference, pollURL); err != nil {
		t.Fatal(err)
	}
	if err = a.verifyBillingPayment(ctx, secondTopupID); err != nil {
		t.Fatalf("second top-up: %v", err)
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["invoice_topup_balance"] != float64(99) || subscription["invoice_topup_debt"] != float64(0) {
		t.Fatalf("new top-up did not cover refunded usage: %d %v", status, subscription)
	}
	providerReference, providerAmount = reference, "3.00"
	providerStatus = "Refunded"
	if err = a.verifyBillingPayment(ctx, billingID); err != nil {
		t.Fatalf("refund: %v", err)
	}
	var restoredUntil, trialUntil time.Time
	if err = a.DB.QueryRow(ctx, `SELECT plan,plan_expires_at,trial_expires_at FROM businesses WHERE id=$1`, businessID).Scan(&paidPlan, &restoredUntil, &trialUntil); err != nil {
		t.Fatal(err)
	}
	if paidPlan != "starter" || !restoredUntil.Equal(trialUntil) {
		t.Fatal("refund did not restore the remaining trial")
	}
	if _, err = a.DB.Exec(ctx, `UPDATE businesses SET trial_started_at=now()-interval '169 hours',trial_expires_at=now()-interval '1 hour',plan_expires_at=now()-interval '1 hour' WHERE id=$1`, businessID); err != nil {
		t.Fatal(err)
	}
	status, subscription, _ = request("GET", "/api/v1/subscription", nil, aCookie)
	if status != 200 || subscription["plan"] != "free" || subscription["trial_active"] != false || subscription["product_limit"] != float64(5) {
		t.Fatalf("expired trial did not fall back to Free: %d %v", status, subscription)
	}
	resetToken := randomHex(32)
	if _, err = a.DB.Exec(ctx, `INSERT INTO password_reset_tokens(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '30 minutes')`, hash(resetToken), userID); err != nil {
		t.Fatal(err)
	}
	status, _, _ = request("POST", "/api/v1/auth/password-reset/confirm", map[string]any{"token": resetToken, "password": "A-new-long-test-password1!"}, nil)
	if status != 200 {
		t.Fatalf("password reset failed: %d", status)
	}
	status, _, _ = request("GET", "/api/v1/me", nil, aCookie)
	if status != 401 {
		t.Fatalf("old session survived password reset: %d", status)
	}
}
