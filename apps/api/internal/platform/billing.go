package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const paynowEndpoint = "https://www.paynow.co.zw/interface/initiatetransaction"

var paynowHTTP = &http.Client{
	Timeout:       15 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type paynowField struct{ key, value string }

func paynowHash(fields []paynowField, key string) string {
	var value strings.Builder
	for _, field := range fields {
		if !strings.EqualFold(field.key, "hash") {
			value.WriteString(field.value)
		}
	}
	value.WriteString(key)
	sum := sha512.Sum512([]byte(value.String()))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// Paynow signs values in wire order, not alphabetical form order.
func parsePaynow(body string, key string) (map[string]string, error) {
	fields := []paynowField{}
	values := map[string]string{}
	for _, pair := range strings.Split(body, "&") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("malformed Paynow response")
		}
		name, err := url.QueryUnescape(parts[0])
		if err != nil {
			return nil, err
		}
		value, err := url.QueryUnescape(parts[1])
		if err != nil {
			return nil, err
		}
		name = strings.ToLower(name)
		if _, exists := values[name]; exists {
			return nil, errors.New("duplicate Paynow field")
		}
		values[name] = value
		fields = append(fields, paynowField{name, value})
	}
	provided, err := hex.DecodeString(values["hash"])
	if err != nil || len(provided) != sha512.Size {
		return nil, errors.New("invalid Paynow hash")
	}
	computed, _ := hex.DecodeString(paynowHash(fields, key))
	if !hmac.Equal(provided, computed) {
		return nil, errors.New("invalid Paynow signature")
	}
	return values, nil
}

func paynowURL(raw string, browser bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Hostname() != "www.paynow.co.zw" || u.Fragment != "" {
		return false
	}
	if browser {
		return strings.HasPrefix(strings.ToLower(u.Path), "/payment/")
	}
	return strings.EqualFold(u.Path, "/interface/checkpayment/") && u.Query().Get("guid") != ""
}

func encodePaynow(fields []paynowField, key string) string {
	fields = append(append([]paynowField{}, fields...), paynowField{"hash", paynowHash(fields, key)})
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, url.QueryEscape(field.key)+"="+url.QueryEscape(field.value))
	}
	return strings.Join(parts, "&")
}

func paynowRequest(ctx context.Context, target string, body string, key string) (map[string]string, error) {
	var request *http.Request
	var err error
	if body == "" {
		if !paynowURL(target, false) {
			return nil, errors.New("invalid Paynow poll URL")
		}
		request, err = http.NewRequestWithContext(ctx, http.MethodPost, target, nil)
	} else {
		request, err = http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(body))
		if err == nil {
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return nil, err
	}
	response, err := paynowHTTP.Do(request)
	if err != nil {
		return nil, errors.New("Paynow request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Paynow HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil || len(data) > 16384 {
		return nil, errors.New("invalid Paynow response size")
	}
	return parsePaynow(string(data), key)
}

func billingPrice(plan string, months int) int64 {
	monthly := map[string]int64{"starter": 300, "business": 500}[plan]
	if months == 1 {
		return monthly
	}
	if months == 12 {
		return monthly * 12
	}
	return 0
}

func invoiceTopupPrice(quantity int) int64 {
	switch quantity {
	case 30:
		return 100
	case 100:
		return 200
	default:
		return 0
	}
}

func (a *App) checkout(w http.ResponseWriter, r *http.Request) {
	if who(r).Role != "owner" {
		fail(w, 403, "Business owner access required")
		return
	}
	var input struct {
		Plan          string `json:"plan"`
		Months        int    `json:"months"`
		TopupInvoices int    `json:"topup_invoices"`
	}
	if !decode(w, r, &input) {
		return
	}
	kind, price := "subscription", billingPrice(input.Plan, input.Months)
	if input.TopupInvoices != 0 {
		kind, price = "invoice_topup", invoiceTopupPrice(input.TopupInvoices)
		if input.Plan != "" || input.Months != 0 {
			price = 0
		}
	}
	if price == 0 {
		fail(w, 400, "Choose a plan for 1 or 12 months, or 30 or 100 invoice top-ups")
		return
	}
	id, key := os.Getenv("PAYNOW_INTEGRATION_ID"), os.Getenv("PAYNOW_INTEGRATION_KEY")
	if id == "" || key == "" || !strings.HasPrefix(a.PublicURL, "https://") {
		fail(w, 503, "Paynow checkout is not configured")
		return
	}
	if _, err := strconv.Atoi(id); err != nil {
		fail(w, 503, "Paynow checkout is not configured")
		return
	}
	if kind == "subscription" {
		var currentPlan string
		var trialActive bool
		if err := a.DB.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,COALESCE(trial_expires_at>now() AND plan=trial_plan AND plan_expires_at=trial_expires_at,false) FROM businesses WHERE id=$1`, who(r).BusinessID).Scan(&currentPlan, &trialActive); err != nil {
			unavailable(w, r, err)
			return
		}
		if currentPlan == "business" && input.Plan == "starter" && !trialActive {
			fail(w, 409, "Wait until the Business plan expires before choosing Starter")
			return
		}
	}
	paymentID := newID()
	reference := "LP-" + strings.ReplaceAll(paymentID, "-", "")
	var plan any = input.Plan
	var months any = input.Months
	info := "LinkPay " + input.Plan + " plan"
	if kind == "invoice_topup" {
		plan, months = nil, nil
		info = fmt.Sprintf("LinkPay %d invoice top-up", input.TopupInvoices)
	}
	_, err := a.DB.Exec(r.Context(), `INSERT INTO billing_payments(id,business_id,created_by,reference,kind,plan,months,invoice_quantity,amount_minor) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, paymentID, who(r).BusinessID, who(r).UserID, reference, kind, plan, months, input.TopupInvoices, price)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	returnURL := a.PublicURL + "/billing/return?payment=" + paymentID
	resultURL := a.PublicURL + "/api/v1/billing/paynow/result"
	fields := []paynowField{{"id", id}, {"reference", reference}, {"amount", money(price)}, {"additionalinfo", info}, {"returnurl", returnURL}, {"resulturl", resultURL}, {"merchanttrace", strings.ReplaceAll(paymentID, "-", "")}, {"status", "Message"}}
	response, err := paynowRequest(r.Context(), paynowEndpoint, encodePaynow(fields, key), key)
	if err != nil || !strings.EqualFold(response["status"], "ok") || !paynowURL(response["browserurl"], true) || !paynowURL(response["pollurl"], false) {
		fail(w, 502, "Could not start Paynow checkout")
		return
	}
	_, err = a.DB.Exec(r.Context(), `UPDATE billing_payments SET status='pending',browser_url=$2,poll_url=$3,updated_at=now() WHERE id=$1`, paymentID, response["browserurl"], response["pollurl"])
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": paymentID, "checkout_url": response["browserurl"]})
}

func (a *App) paynowResult(w http.ResponseWriter, r *http.Request) {
	key := os.Getenv("PAYNOW_INTEGRATION_KEY")
	if key == "" {
		fail(w, 503, "Billing unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 400, "Invalid update")
		return
	}
	message, err := parsePaynow(string(data), key)
	if err != nil || message["reference"] == "" {
		fail(w, 400, "Invalid update")
		return
	}
	var paymentID string
	err = a.DB.QueryRow(r.Context(), `SELECT id::text FROM billing_payments WHERE reference=$1`, message["reference"]).Scan(&paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Payment not found")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = a.verifyBillingPayment(r.Context(), paymentID); err != nil {
		logError(r, err)
		fail(w, 503, "Payment verification unavailable")
		return
	}
	w.WriteHeader(204)
}

func (a *App) refreshBillingPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Payment not found")
		return
	}
	var exists bool
	err := a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM billing_payments WHERE id=$1 AND business_id=$2)`, id, who(r).BusinessID).Scan(&exists)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if !exists {
		fail(w, 404, "Payment not found")
		return
	}
	if err = a.verifyBillingPayment(r.Context(), id); err != nil {
		logError(r, err)
		fail(w, 503, "Payment verification unavailable")
		return
	}
	a.billingPayments(w, r)
}

func (a *App) verifyBillingPayment(ctx context.Context, id string) error {
	var pollURL, reference string
	var price int64
	err := a.DB.QueryRow(ctx, `SELECT poll_url,reference,amount_minor FROM billing_payments WHERE id=$1`, id).Scan(&pollURL, &reference, &price)
	if err != nil {
		return err
	}
	key := os.Getenv("PAYNOW_INTEGRATION_KEY")
	if key == "" {
		return errors.New("Paynow is not configured")
	}
	if pollURL == "" {
		fields := []paynowField{{"id", os.Getenv("PAYNOW_INTEGRATION_ID")}, {"merchanttrace", strings.TrimPrefix(reference, "LP-")}, {"status", "Message"}}
		traced, traceErr := paynowRequest(ctx, "https://www.paynow.co.zw/interface/trace", encodePaynow(fields, key), key)
		if traceErr != nil {
			return traceErr
		}
		if traced["status"] == "NotFound" {
			return errors.New("Paynow transaction not found yet")
		}
		if traced["reference"] != reference || traced["amount"] != money(price) || !paynowURL(traced["pollurl"], false) {
			return errors.New("Paynow trace did not match payment")
		}
		pollURL = traced["pollurl"]
		if _, err = a.DB.Exec(ctx, `UPDATE billing_payments SET poll_url=$2,status='pending',updated_at=now() WHERE id=$1 AND poll_url=''`, id, pollURL); err != nil {
			return err
		}
	}
	status, err := paynowRequest(ctx, pollURL, "", key)
	if err != nil {
		return err
	}
	if status["reference"] != reference || status["amount"] != money(price) {
		return errors.New("Paynow payment details do not match")
	}
	if status["pollurl"] != "" && status["pollurl"] != pollURL {
		return errors.New("Paynow poll URL changed")
	}
	newStatus := strings.ToLower(status["status"])
	switch newStatus {
	case "created", "sent", "awaiting delivery", "delivered":
		newStatus = "pending"
	case "paid", "cancelled", "disputed", "refunded", "failed":
	default:
		return fmt.Errorf("unexpected Paynow status: %s", newStatus)
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var businessID, kind, plan, current string
	var months, invoiceQuantity int
	var granted *time.Time
	err = tx.QueryRow(ctx, `SELECT business_id::text,kind,COALESCE(plan,''),COALESCE(months,0),invoice_quantity,status,granted_until FROM billing_payments WHERE id=$1 FOR UPDATE`, id).Scan(&businessID, &kind, &plan, &months, &invoiceQuantity, &current, &granted)
	if err != nil {
		return err
	}
	if current == "paid" && newStatus != "refunded" && newStatus != "disputed" {
		return tx.Commit(ctx)
	}
	if current == "refunded" || current == "disputed" {
		return tx.Commit(ctx)
	}
	if kind == "invoice_topup" {
		var locked string
		if err = tx.QueryRow(ctx, `SELECT id::text FROM businesses WHERE id=$1 FOR UPDATE`, businessID).Scan(&locked); err != nil {
			return err
		}
		if newStatus == "paid" && current != "paid" {
			if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(id,business_id,action,subject_id,details) VALUES($1,$2,'invoice_topup.paid',$3,jsonb_build_object('quantity',$4::integer))`, newID(), businessID, id, invoiceQuantity); err != nil {
				return err
			}
		}
		if (newStatus == "refunded" || newStatus == "disputed") && current == "paid" {
			if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(id,business_id,action,subject_id,details) VALUES($1,$2,$3,$4,jsonb_build_object('quantity',$5::integer))`, newID(), businessID, "invoice_topup."+newStatus, id, invoiceQuantity); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE billing_payments SET status=$2,paynow_reference=$3,updated_at=now() WHERE id=$1`, id, newStatus, status["paynowreference"])
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if newStatus == "paid" && current != "paid" {
		var expires *time.Time
		if err = tx.QueryRow(ctx, `SELECT plan_expires_at FROM businesses WHERE id=$1 FOR UPDATE`, businessID).Scan(&expires); err != nil {
			return err
		}
		start := time.Now().UTC()
		if expires != nil && expires.After(start) {
			start = *expires
		}
		until := start.AddDate(0, months, 0)
		if _, err = tx.Exec(ctx, `UPDATE businesses SET plan=$2,plan_expires_at=$3 WHERE id=$1`, businessID, plan, until); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE billing_payments SET granted_until=$2 WHERE id=$1`, id, until); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(id,business_id,action,subject_id,details) VALUES($1,$2,'subscription.paid',$3,jsonb_build_object('plan',$4::text,'until',$5::timestamptz))`, newID(), businessID, id, plan, until); err != nil {
			return err
		}
	}
	if (newStatus == "refunded" || newStatus == "disputed") && current == "paid" && granted != nil {
		var previousPlan string
		var previousUntil time.Time
		previousErr := tx.QueryRow(ctx, `SELECT plan,granted_until FROM billing_payments WHERE business_id=$1 AND id<>$2 AND status='paid' AND granted_until>now() ORDER BY granted_until DESC LIMIT 1`, businessID, id).Scan(&previousPlan, &previousUntil)
		if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
			return previousErr
		}
		if errors.Is(previousErr, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `UPDATE businesses SET plan=CASE WHEN trial_expires_at>now() THEN trial_plan ELSE 'free' END,plan_expires_at=CASE WHEN trial_expires_at>now() THEN trial_expires_at ELSE NULL END WHERE id=$1 AND plan_expires_at=$2`, businessID, *granted)
		} else {
			_, err = tx.Exec(ctx, `UPDATE businesses SET plan=$3,plan_expires_at=$4 WHERE id=$1 AND plan_expires_at=$2`, businessID, *granted, previousPlan, previousUntil)
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(id,business_id,action,subject_id) VALUES($1,$2,$3,$4)`, newID(), businessID, "subscription."+newStatus, id); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE billing_payments SET status=$2,paynow_reference=$3,updated_at=now() WHERE id=$1`, id, newStatus, status["paynowreference"])
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *App) billingPayments(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,kind,COALESCE(plan,''),COALESCE(months,0),invoice_quantity,amount_minor,status,created_at,COALESCE(granted_until::text,'') FROM billing_payments WHERE business_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, kind, plan, status, until string
		var months, invoiceQuantity int
		var amount int64
		var created time.Time
		if err = rows.Scan(&id, &kind, &plan, &months, &invoiceQuantity, &amount, &status, &created, &until); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, map[string]any{"id": id, "kind": kind, "plan": plan, "months": months, "invoice_quantity": invoiceQuantity, "amount_minor": amount, "status": status, "created_at": created, "granted_until": until})
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}

// Reconcile old attempts in case a callback was lost. Paynow discourages routine polling,
// so this only checks attempts older than a day and spaces repeat checks by a day.
func (a *App) ReconcileBilling(ctx context.Context) error {
	rows, err := a.DB.Query(ctx, `SELECT id::text FROM billing_payments WHERE status IN ('created','pending') AND updated_at<now()-interval '24 hours' ORDER BY updated_at LIMIT 20`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err = a.verifyBillingPayment(ctx, id); err != nil {
			_, _ = a.DB.Exec(ctx, `UPDATE billing_payments SET updated_at=now() WHERE id=$1`, id)
			failures = append(failures, fmt.Errorf("reconcile payment %s: %w", id, err))
		}
	}
	return errors.Join(failures...)
}
