package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-pdf/fpdf"
	"github.com/jackc/pgx/v5"
)

type invoiceItem struct {
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	UnitMinor   int64  `json:"unit_minor"`
	TotalMinor  int64  `json:"total_minor"`
}
type invoiceInput struct {
	CustomerID    string        `json:"customer_id"`
	DueAt         string        `json:"due_at"`
	DiscountMinor int64         `json:"discount_minor"`
	TaxMinor      int64         `json:"tax_minor"`
	Notes         string        `json:"notes"`
	Items         []invoiceItem `json:"items"`
}
type invoiceView struct {
	ID            string        `json:"id"`
	Number        int64         `json:"number"`
	CustomerName  string        `json:"customer_name"`
	CustomerEmail string        `json:"customer_email"`
	CustomerPhone string        `json:"customer_phone"`
	Currency      string        `json:"currency"`
	DueAt         string        `json:"due_at"`
	IssuedAt      string        `json:"issued_at"`
	Status        string        `json:"status"`
	SubtotalMinor int64         `json:"subtotal_minor"`
	DiscountMinor int64         `json:"discount_minor"`
	TaxMinor      int64         `json:"tax_minor"`
	TotalMinor    int64         `json:"total_minor"`
	PaidMinor     int64         `json:"paid_minor"`
	BalanceMinor  int64         `json:"balance_minor"`
	Notes         string        `json:"notes"`
	Items         []invoiceItem `json:"items,omitempty"`
}

const invoiceSelect = `SELECT i.id::text,i.number,c.name,c.email,c.phone,i.currency,i.due_at,i.issued_at,i.status,i.subtotal_minor,i.discount_minor,i.tax_minor,i.total_minor,COALESCE(p.paid,0),i.notes,(timezone(b.timezone,now())::date)::text FROM invoices i JOIN customers c ON c.id=i.customer_id AND c.business_id=i.business_id JOIN businesses b ON b.id=i.business_id LEFT JOIN LATERAL (SELECT sum(amount_minor) paid FROM payments WHERE invoice_id=i.id AND business_id=i.business_id) p ON true`

type scanner interface{ Scan(...any) error }

func scanInvoice(row scanner) (invoiceView, error) {
	var v invoiceView
	var issued *time.Time
	var due time.Time
	var rawStatus, today string
	err := row.Scan(&v.ID, &v.Number, &v.CustomerName, &v.CustomerEmail, &v.CustomerPhone, &v.Currency, &due, &issued, &rawStatus, &v.SubtotalMinor, &v.DiscountMinor, &v.TaxMinor, &v.TotalMinor, &v.PaidMinor, &v.Notes, &today)
	if err != nil {
		return v, err
	}
	v.DueAt = due.Format("2006-01-02")
	if issued != nil {
		v.IssuedAt = issued.UTC().Format(time.RFC3339)
	}
	v.BalanceMinor = v.TotalMinor - v.PaidMinor
	v.Status = invoiceStatus(v.IssuedAt, rawStatus == "void", v.PaidMinor, v.TotalMinor, v.DueAt, today)
	return v, nil
}
func invoiceStatus(issued string, void bool, paid, total int64, due, today string) string {
	if void {
		return "void"
	}
	if issued == "" {
		return "draft"
	}
	if paid >= total {
		return "paid"
	}
	if today > due {
		return "overdue"
	}
	if paid > 0 {
		return "partially_paid"
	}
	return "sent"
}
func planInvoiceLimit(plan string) int {
	switch plan {
	case "starter":
		return 60
	case "business":
		return 500
	default:
		return 30
	}
}

type invoiceQuotaReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func invoiceQuota(ctx context.Context, db invoiceQuotaReader, businessID string) (int, int, error) {
	var monthlyUsed, topupBalance int
	err := db.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM invoices WHERE business_id=$1 AND quota_source='subscription' AND created_at>=date_trunc('month',now())),
 (SELECT COALESCE(sum(invoice_quantity),0) FROM billing_payments WHERE business_id=$1 AND kind='invoice_topup' AND status='paid')
 -(SELECT count(*) FROM invoices WHERE business_id=$1 AND quota_source='invoice_topup')`, businessID).Scan(&monthlyUsed, &topupBalance)
	return monthlyUsed, topupBalance, err
}

func (a *App) createInvoice(w http.ResponseWriter, r *http.Request) {
	var in invoiceInput
	if !decode(w, r, &in) {
		return
	}
	due, err := time.Parse("2006-01-02", in.DueAt)
	if err != nil || due.Before(time.Now().UTC().AddDate(0, 0, -1)) || len(in.Items) == 0 || len(in.Items) > 50 || len(in.Notes) > 2000 || in.DiscountMinor < 0 || in.TaxMinor < 0 {
		fail(w, 400, "Invalid invoice details")
		return
	}
	if !validID(in.CustomerID) {
		fail(w, 400, "Customer not found")
		return
	}
	var subtotal int64
	for i := range in.Items {
		it := &in.Items[i]
		it.Description = strings.TrimSpace(it.Description)
		if len(it.Description) < 2 || len(it.Description) > 500 || it.Quantity < 1 || it.Quantity > 100000 || it.UnitMinor < 0 || it.UnitMinor > 1000000000 {
			fail(w, 400, "Invalid invoice item")
			return
		}
		it.TotalMinor = it.Quantity * it.UnitMinor
		if subtotal > 1000000000000-it.TotalMinor {
			fail(w, 400, "Invoice total is too large")
			return
		}
		subtotal += it.TotalMinor
	}
	if in.DiscountMinor > subtotal || in.TaxMinor > 1000000000000-subtotal {
		fail(w, 400, "Invalid discount or tax")
		return
	}
	total := subtotal - in.DiscountMinor + in.TaxMinor
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var plan, currency string
	var number int64
	err = tx.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,currency,next_invoice_number FROM businesses WHERE id=$1 FOR UPDATE`, who(r).BusinessID).Scan(&plan, &currency, &number)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	monthlyUsed, topupBalance, err := invoiceQuota(r.Context(), tx, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	quotaSource := "subscription"
	if monthlyUsed >= planInvoiceLimit(plan) {
		if topupBalance <= 0 {
			fail(w, 403, "Monthly invoice limit reached. Buy an invoice top-up to continue.")
			return
		}
		quotaSource = "invoice_topup"
	}
	var exists bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM customers WHERE id=$1 AND business_id=$2)`, in.CustomerID, who(r).BusinessID).Scan(&exists)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if !exists {
		fail(w, 400, "Customer not found")
		return
	}
	id := newID()
	_, err = tx.Exec(r.Context(), `INSERT INTO invoices(id,business_id,customer_id,number,currency,due_at,subtotal_minor,discount_minor,tax_minor,total_minor,notes,quota_source) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, who(r).BusinessID, in.CustomerID, number, currency, in.DueAt, subtotal, in.DiscountMinor, in.TaxMinor, total, in.Notes, quotaSource)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	for _, it := range in.Items {
		_, err = tx.Exec(r.Context(), `INSERT INTO invoice_items(id,business_id,invoice_id,description,quantity,unit_minor,total_minor) VALUES($1,$2,$3,$4,$5,$6,$7)`, newID(), who(r).BusinessID, id, it.Description, it.Quantity, it.UnitMinor, it.TotalMinor)
		if err != nil {
			unavailable(w, r, err)
			return
		}
	}
	_, err = tx.Exec(r.Context(), `UPDATE businesses SET next_invoice_number=next_invoice_number+1 WHERE id=$1`, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,actor_id,action,subject_id) VALUES($1,$2,$3,'invoice.created',$4)`, newID(), who(r).BusinessID, who(r).UserID, id)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "number": number})
}
func (a *App) loadInvoice(r *http.Request, id, businessID string, items bool) (invoiceView, error) {
	if !validID(id) {
		return invoiceView{}, pgx.ErrNoRows
	}
	v, err := scanInvoice(a.DB.QueryRow(r.Context(), invoiceSelect+` WHERE i.id=$1 AND i.business_id=$2`, id, businessID))
	if err != nil {
		return v, err
	}
	if items {
		rows, err := a.DB.Query(r.Context(), `SELECT description,quantity,unit_minor,total_minor FROM invoice_items WHERE business_id=$1 AND invoice_id=$2 ORDER BY id`, businessID, id)
		if err != nil {
			return v, err
		}
		defer rows.Close()
		v.Items = []invoiceItem{}
		for rows.Next() {
			var it invoiceItem
			if err = rows.Scan(&it.Description, &it.Quantity, &it.UnitMinor, &it.TotalMinor); err != nil {
				return v, err
			}
			v.Items = append(v.Items, it)
		}
		if rows.Err() != nil {
			return v, rows.Err()
		}
	}
	return v, nil
}
func (a *App) invoices(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), invoiceSelect+` WHERE i.business_id=$1 ORDER BY i.created_at DESC,i.id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []invoiceView{}
	for rows.Next() {
		v, e := scanInvoice(rows)
		if e != nil {
			unavailable(w, r, e)
			return
		}
		out = append(out, v)
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}
func (a *App) invoice(w http.ResponseWriter, r *http.Request) {
	v, err := a.loadInvoice(r, chi.URLParam(r, "id"), who(r).BusinessID, true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Invoice not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	writeJSON(w, 200, v)
}
func (a *App) sendInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice not found")
		return
	}
	token := a.invoiceToken(id)
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	ct, err := tx.Exec(r.Context(), `UPDATE invoices SET status='sent',issued_at=COALESCE(issued_at,now()),public_token_hash=$1 WHERE id=$2 AND business_id=$3 AND status!='void'`, hash(token), id, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "Invoice not found")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,actor_id,action,subject_id) VALUES($1,$2,$3,'invoice.sent',$4)`, newID(), who(r).BusinessID, who(r).UserID, id)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"url": a.PublicURL + "/invoice/" + token})
}
func (a *App) invoiceShareLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice link not available")
		return
	}
	token := a.invoiceToken(id)
	var exists bool
	err := a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM invoices WHERE id=$1 AND business_id=$2 AND status='sent' AND public_token_hash=$3)`, id, who(r).BusinessID, hash(token)).Scan(&exists)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if !exists {
		fail(w, 404, "Invoice link not available")
		return
	}
	writeJSON(w, 200, map[string]string{"url": a.PublicURL + "/invoice/" + token})
}
func (a *App) voidInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice not found")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	ct, err := tx.Exec(r.Context(), `UPDATE invoices SET status='void',public_token_hash=NULL WHERE id=$1 AND business_id=$2 AND status!='void' AND NOT EXISTS(SELECT 1 FROM payments WHERE invoice_id=$1 AND business_id=$2)`, id, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 409, "Invoice cannot be voided after payment, or it was not found")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,actor_id,action,subject_id) VALUES($1,$2,$3,'invoice.voided',$4)`, newID(), who(r).BusinessID, who(r).UserID, id)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	w.WriteHeader(204)
}

type paymentInput struct {
	AmountMinor    int64  `json:"amount_minor"`
	Method         string `json:"method"`
	Reference      string `json:"reference"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (a *App) recordPayment(w http.ResponseWriter, r *http.Request) {
	var in paymentInput
	if !decode(w, r, &in) {
		return
	}
	in.Method = strings.TrimSpace(in.Method)
	in.Reference = strings.TrimSpace(in.Reference)
	if in.AmountMinor <= 0 || in.AmountMinor > 1000000000000 || len(in.Method) < 2 || len(in.Method) > 50 || len(in.Reference) > 120 || len(in.IdempotencyKey) < 16 || len(in.IdempotencyKey) > 100 {
		fail(w, 400, "Invalid payment")
		return
	}
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice not found")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var total int64
	var status string
	err = tx.QueryRow(r.Context(), `SELECT total_minor,status FROM invoices WHERE id=$1 AND business_id=$2 FOR UPDATE`, id, who(r).BusinessID).Scan(&total, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Invoice not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if status != "sent" {
		fail(w, 409, "Only sent invoices accept payments")
		return
	}
	var prior int64
	var priorMethod, priorReference string
	err = tx.QueryRow(r.Context(), `SELECT amount_minor,method,reference FROM payments WHERE invoice_id=$1 AND business_id=$2 AND idempotency_key=$3`, id, who(r).BusinessID, in.IdempotencyKey).Scan(&prior, &priorMethod, &priorReference)
	if err == nil {
		if prior != in.AmountMinor || priorMethod != in.Method || priorReference != in.Reference {
			fail(w, 409, "Idempotency key already used with different payment details")
			return
		}
		writeJSON(w, 200, map[string]any{"already_recorded": true})
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		unavailable(w, r, err)
		return
	}
	var paid int64
	err = tx.QueryRow(r.Context(), `SELECT COALESCE(sum(amount_minor),0) FROM payments WHERE invoice_id=$1 AND business_id=$2`, id, who(r).BusinessID).Scan(&paid)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if in.AmountMinor > total-paid {
		fail(w, 409, "Payment exceeds outstanding balance")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO payments(id,business_id,invoice_id,amount_minor,method,reference,idempotency_key,recorded_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, newID(), who(r).BusinessID, id, in.AmountMinor, in.Method, in.Reference, in.IdempotencyKey, who(r).UserID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,actor_id,action,subject_id,details) VALUES($1,$2,$3,'payment.recorded',$4,jsonb_build_object('amount_minor',$5::bigint,'method',$6::text))`, newID(), who(r).BusinessID, who(r).UserID, id, in.AmountMinor, in.Method)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]int64{"balance_minor": total - paid - in.AmountMinor})
}
func (a *App) publicInvoice(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != 64 {
		fail(w, 404, "Invoice not found")
		return
	}
	var id, businessID string
	err := a.DB.QueryRow(r.Context(), `SELECT id::text,business_id::text FROM invoices WHERE public_token_hash=$1 AND status='sent'`, hash(token)).Scan(&id, &businessID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Invoice not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	v, err := a.loadInvoice(r, id, businessID, true)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	var b businessView
	b, err = a.readBusiness(r, businessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	v.CustomerEmail = "" // The bearer link does not expose contact details.
	v.CustomerPhone = ""
	writeJSON(w, 200, map[string]any{"invoice": v, "business": map[string]string{"name": b.Name, "email": b.Email, "whatsapp": b.Whatsapp}})
}
func money(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }
func (a *App) invoicePDF(w http.ResponseWriter, r *http.Request) {
	v, err := a.loadInvoice(r, chi.URLParam(r, "id"), who(r).BusinessID, true)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Invoice not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	b, err := a.readBusiness(r, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(18, 18, 18)
	p.AddPage()
	p.SetFont("Arial", "B", 20)
	p.Cell(0, 12, b.Name)
	p.Ln(18)
	p.SetFont("Arial", "B", 16)
	p.Cell(0, 10, fmt.Sprintf("INVOICE #%06d", v.Number))
	p.Ln(12)
	p.SetFont("Arial", "", 11)
	p.Cell(0, 7, "Bill to: "+v.CustomerName)
	p.Ln(7)
	p.Cell(0, 7, "Due: "+v.DueAt+"    Status: "+strings.ToUpper(v.Status))
	p.Ln(12)
	p.SetFont("Arial", "B", 10)
	p.CellFormat(110, 8, "Description", "B", 0, "L", false, 0, "")
	p.CellFormat(25, 8, "Qty", "B", 0, "R", false, 0, "")
	p.CellFormat(40, 8, "Total", "B", 1, "R", false, 0, "")
	p.SetFont("Arial", "", 10)
	for _, it := range v.Items {
		p.CellFormat(110, 9, it.Description, "B", 0, "L", false, 0, "")
		p.CellFormat(25, 9, fmt.Sprint(it.Quantity), "B", 0, "R", false, 0, "")
		p.CellFormat(40, 9, v.Currency+" "+money(it.TotalMinor), "B", 1, "R", false, 0, "")
	}
	p.Ln(8)
	for _, line := range []string{"Subtotal: " + v.Currency + " " + money(v.SubtotalMinor), "Discount: " + v.Currency + " " + money(v.DiscountMinor), "Tax: " + v.Currency + " " + money(v.TaxMinor), "Total: " + v.Currency + " " + money(v.TotalMinor), "Paid: " + v.Currency + " " + money(v.PaidMinor), "Balance: " + v.Currency + " " + money(v.BalanceMinor)} {
		p.CellFormat(0, 8, line, "", 1, "R", false, 0, "")
	}
	if v.Notes != "" {
		p.Ln(8)
		p.MultiCell(0, 7, "Notes: "+v.Notes, "", "L", false)
	}
	var buf bytes.Buffer
	if err = p.Output(&buf); err != nil {
		unavailable(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%06d.pdf"`, v.Number))
	w.WriteHeader(200)
	_, _ = w.Write(buf.Bytes())
}
