package platform

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (a *App) updateEnquiryStage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Stage string `json:"stage"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Stage != "new" && in.Stage != "contacted" && in.Stage != "quoted" && in.Stage != "won" && in.Stage != "lost" {
		fail(w, 400, "Invalid enquiry stage")
		return
	}
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Enquiry not found")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var old string
	if err = tx.QueryRow(r.Context(), `SELECT stage FROM enquiries WHERE id=$1 AND business_id=$2 FOR UPDATE`, id, who(r).BusinessID).Scan(&old); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Enquiry not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if old != in.Stage {
		if _, err = tx.Exec(r.Context(), `UPDATE enquiries SET stage=$1,updated_at=now() WHERE id=$2 AND business_id=$3`, in.Stage, id, who(r).BusinessID); err != nil {
			unavailable(w, r, err)
			return
		}
		if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,actor_id,action,subject_id,details) VALUES($1,$2,$3,'enquiry.stage_changed',$4,jsonb_build_object('from',$5::text,'to',$6::text))`, newID(), who(r).BusinessID, who(r).UserID, id, old, in.Stage); err != nil {
			unavailable(w, r, err)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"stage": in.Stage})
}

type quoteView struct {
	ID           string `json:"id"`
	EnquiryID    string `json:"enquiry_id"`
	CustomerName string `json:"customer_name"`
	Description  string `json:"description"`
	AmountMinor  int64  `json:"amount_minor"`
	Currency     string `json:"currency"`
	ExpiresAt    string `json:"expires_at"`
	Status       string `json:"status"`
	URL          string `json:"url,omitempty"`
	InvoiceID    string `json:"invoice_id,omitempty"`
}

const quoteSelect = `SELECT q.id::text,q.enquiry_id::text,e.name,q.description,q.amount_minor,q.currency,q.expires_at::text,CASE WHEN q.status='open' AND q.expires_at<timezone(b.timezone,now())::date THEN 'expired' ELSE q.status END,COALESCE(q.invoice_id::text,'') FROM quotes q JOIN enquiries e ON e.id=q.enquiry_id AND e.business_id=q.business_id JOIN businesses b ON b.id=q.business_id`

func scanQuote(row scanner) (quoteView, error) {
	var q quoteView
	err := row.Scan(&q.ID, &q.EnquiryID, &q.CustomerName, &q.Description, &q.AmountMinor, &q.Currency, &q.ExpiresAt, &q.Status, &q.InvoiceID)
	return q, err
}

func (a *App) quoteToken(id string) string { return a.invoiceToken("quote:" + id) }

func (a *App) createQuote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		EnquiryID   string `json:"enquiry_id"`
		Description string `json:"description"`
		AmountMinor int64  `json:"amount_minor"`
		ExpiresAt   string `json:"expires_at"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Description = strings.TrimSpace(in.Description)
	expires, err := time.Parse("2006-01-02", in.ExpiresAt)
	if !validID(in.EnquiryID) || len(in.Description) < 2 || len(in.Description) > 2000 || in.AmountMinor <= 0 || in.AmountMinor > 1000000000 || err != nil || expires.Before(time.Now().UTC().Truncate(24*time.Hour)) || expires.After(time.Now().UTC().AddDate(1, 0, 0)) {
		fail(w, 400, "Invalid quote details")
		return
	}
	id := newID()
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var currency string
	if err = tx.QueryRow(r.Context(), `SELECT currency FROM businesses WHERE id=$1 FOR SHARE`, who(r).BusinessID).Scan(&currency); err != nil {
		unavailable(w, r, err)
		return
	}
	var stage string
	if err = tx.QueryRow(r.Context(), `SELECT stage FROM enquiries WHERE id=$1 AND business_id=$2 FOR UPDATE`, in.EnquiryID, who(r).BusinessID).Scan(&stage); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Enquiry not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if stage == "lost" {
		fail(w, 409, "Reopen the enquiry before quoting")
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO quotes(id,business_id,enquiry_id,public_token_hash,description,amount_minor,currency,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, who(r).BusinessID, in.EnquiryID, hash(a.quoteToken(id)), in.Description, in.AmountMinor, currency, in.ExpiresAt); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE enquiries SET stage='quoted',updated_at=now() WHERE id=$1 AND business_id=$2`, in.EnquiryID, who(r).BusinessID); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,actor_id,action,subject_id) VALUES($1,$2,$3,'quote.created',$4)`, newID(), who(r).BusinessID, who(r).UserID, id); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]string{"id": id, "url": a.PublicURL + "/quote/" + a.quoteToken(id)})
}

func (a *App) quotes(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), quoteSelect+` WHERE q.business_id=$1 ORDER BY q.created_at DESC,q.id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []quoteView{}
	for rows.Next() {
		q, e := scanQuote(rows)
		if e != nil {
			unavailable(w, r, e)
			return
		}
		q.URL = a.PublicURL + "/quote/" + a.quoteToken(q.ID)
		out = append(out, q)
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}

func (a *App) publicQuote(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != 64 {
		fail(w, 404, "Quote not found")
		return
	}
	q, err := scanQuote(a.DB.QueryRow(r.Context(), quoteSelect+` WHERE q.public_token_hash=$1`, hash(token)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Quote not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	var name string
	if err = a.DB.QueryRow(r.Context(), `SELECT b.name FROM quotes q JOIN businesses b ON b.id=q.business_id WHERE q.id=$1`, q.ID).Scan(&name); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"quote": q, "business_name": name})
}

func (a *App) respondQuote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Decision string `json:"decision"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Decision != "accepted" && in.Decision != "declined" {
		fail(w, 400, "Choose accept or decline")
		return
	}
	token := chi.URLParam(r, "token")
	if len(token) != 64 {
		fail(w, 404, "Quote not found")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var id, businessID, enquiryID, status, zone string
	var expires time.Time
	if err = tx.QueryRow(r.Context(), `SELECT q.id::text,q.business_id::text,q.enquiry_id::text,q.status,q.expires_at,b.timezone FROM quotes q JOIN businesses b ON b.id=q.business_id WHERE q.public_token_hash=$1 FOR UPDATE OF q`, hash(token)).Scan(&id, &businessID, &enquiryID, &status, &expires, &zone); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Quote not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if status == in.Decision {
		writeJSON(w, 200, map[string]string{"status": status})
		return
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if status != "open" || expires.Format("2006-01-02") < time.Now().In(location).Format("2006-01-02") {
		fail(w, 409, "Quote is no longer open")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE quotes SET status=$1,responded_at=now() WHERE id=$2`, in.Decision, id); err != nil {
		unavailable(w, r, err)
		return
	}
	if in.Decision == "accepted" {
		if _, err = tx.Exec(r.Context(), `UPDATE enquiries SET stage='won',updated_at=now() WHERE id=$1 AND business_id=$2`, enquiryID, businessID); err != nil {
			unavailable(w, r, err)
			return
		}
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO merchant_notifications(id,business_id,kind,subject_id,title) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, newID(), businessID, "quote_"+in.Decision, id, "Quote "+in.Decision); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,action,subject_id) VALUES($1,$2,$3,$4)`, newID(), businessID, "quote."+in.Decision, id); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]string{"status": in.Decision})
}

func (a *App) notifications(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,kind,subject_id::text,title,created_at::text,read_at IS NOT NULL,email_status FROM merchant_notifications WHERE business_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, kind, subject, title, created, email string
		var read bool
		if err = rows.Scan(&id, &kind, &subject, &title, &created, &read, &email); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, map[string]any{"id": id, "kind": kind, "subject_id": subject, "title": title, "created_at": created, "read": read, "email_status": email})
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}

func (a *App) notificationCounts(w http.ResponseWriter, r *http.Request) {
	var activity, enquiries, quotes int64
	err := a.DB.QueryRow(r.Context(), `SELECT
		(SELECT count(*) FROM merchant_notifications WHERE business_id=$1 AND read_at IS NULL),
		(SELECT count(*) FROM enquiries WHERE business_id=$1 AND stage='new'),
		(SELECT count(*) FROM quotes WHERE business_id=$1 AND status='accepted' AND invoice_id IS NULL)`, who(r).BusinessID).Scan(&activity, &enquiries, &quotes)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, map[string]int64{"activity": activity, "enquiries": enquiries, "quotes": quotes})
}

func (a *App) readNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Notification not found")
		return
	}
	ct, err := a.DB.Exec(r.Context(), `UPDATE merchant_notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1 AND business_id=$2`, id, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "Notification not found")
		return
	}
	w.WriteHeader(204)
}

func (a *App) publicPaymentClaim(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AmountMinor int64  `json:"amount_minor"`
		Reference   string `json:"reference"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Reference = strings.TrimSpace(in.Reference)
	if in.AmountMinor <= 0 || len(in.Reference) > 120 {
		fail(w, 400, "Invalid payment notification")
		return
	}
	token := chi.URLParam(r, "token")
	if len(token) != 64 {
		fail(w, 404, "Invoice not found")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var invoiceID, businessID string
	var total, paid int64
	err = tx.QueryRow(r.Context(), `SELECT i.id::text,i.business_id::text,i.total_minor,COALESCE((SELECT sum(amount_minor) FROM payments WHERE invoice_id=i.id AND business_id=i.business_id),0) FROM invoices i WHERE i.public_token_hash=$1 AND i.status='sent' FOR UPDATE`, hash(token)).Scan(&invoiceID, &businessID, &total, &paid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Invoice not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if in.AmountMinor > total-paid {
		fail(w, 400, "Amount exceeds the outstanding balance")
		return
	}
	var exists bool
	if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM payment_claims WHERE invoice_id=$1 AND status='pending')`, invoiceID).Scan(&exists); err != nil {
		unavailable(w, r, err)
		return
	}
	if exists {
		fail(w, 409, "A payment notification is already pending review")
		return
	}
	id := newID()
	if _, err = tx.Exec(r.Context(), `INSERT INTO payment_claims(id,business_id,invoice_id,amount_minor,reference) VALUES($1,$2,$3,$4,$5)`, id, businessID, invoiceID, in.AmountMinor, in.Reference); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO merchant_notifications(id,business_id,kind,subject_id,title) VALUES($1,$2,'payment_claim',$3,'Customer reported a payment')`, newID(), businessID, id); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO audit_logs(id,business_id,action,subject_id,details) VALUES($1,$2,'payment.reported',$3,jsonb_build_object('amount_minor',$4::bigint))`, newID(), businessID, invoiceID, in.AmountMinor); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]string{"status": "pending_review"})
}

func (a *App) paymentClaims(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice not found")
		return
	}
	var exists bool
	err := a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM invoices WHERE id=$1 AND business_id=$2)`, id, who(r).BusinessID).Scan(&exists)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if !exists {
		fail(w, 404, "Invoice not found")
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,amount_minor,reference,status,created_at::text FROM payment_claims WHERE invoice_id=$1 AND business_id=$2 ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, id, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var claimID, ref, status, created string
		var amount int64
		if err = rows.Scan(&claimID, &amount, &ref, &status, &created); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, map[string]any{"id": claimID, "amount_minor": amount, "reference": ref, "status": status, "created_at": created})
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}
func (a *App) dismissPaymentClaim(w http.ResponseWriter, r *http.Request) {
	id, claimID := chi.URLParam(r, "id"), chi.URLParam(r, "claimID")
	if !validID(id) || !validID(claimID) {
		fail(w, 404, "Payment notification not found")
		return
	}
	ct, err := a.DB.Exec(r.Context(), `UPDATE payment_claims SET status='dismissed',resolved_at=now() WHERE id=$1 AND invoice_id=$2 AND business_id=$3 AND status='pending'`, claimID, id, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "Payment notification not found")
		return
	}
	w.WriteHeader(204)
}

func (a *App) followups(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT f.id::text,f.customer_id::text,c.name,f.due_on::text,f.note,f.completed_at::text FROM customer_followups f JOIN customers c ON c.id=f.customer_id AND c.business_id=f.business_id WHERE f.business_id=$1 ORDER BY f.completed_at NULLS FIRST,f.due_on,f.id LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, customerID, name, due, note string
		var completed *string
		if err = rows.Scan(&id, &customerID, &name, &due, &note, &completed); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, map[string]any{"id": id, "customer_id": customerID, "customer_name": name, "due_on": due, "note": note, "completed_at": completed})
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}
func (a *App) createFollowup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CustomerID string `json:"customer_id"`
		DueOn      string `json:"due_on"`
		Note       string `json:"note"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	due, err := time.Parse("2006-01-02", in.DueOn)
	if !validID(in.CustomerID) || err != nil || due.Before(time.Now().UTC().AddDate(0, 0, -1)) || due.After(time.Now().UTC().AddDate(2, 0, 0)) || len(in.Note) < 2 || len(in.Note) > 500 {
		fail(w, 400, "Invalid follow-up")
		return
	}
	id := newID()
	ct, err := a.DB.Exec(r.Context(), `INSERT INTO customer_followups(id,business_id,customer_id,due_on,note) SELECT $1,$2,id,$4,$5 FROM customers WHERE id=$3 AND business_id=$2`, id, who(r).BusinessID, in.CustomerID, in.DueOn, in.Note)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "Customer not found")
		return
	}
	writeJSON(w, 201, map[string]string{"id": id})
}
func (a *App) completeFollowup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Follow-up not found")
		return
	}
	ct, err := a.DB.Exec(r.Context(), `UPDATE customer_followups SET completed_at=now() WHERE id=$1 AND business_id=$2 AND completed_at IS NULL`, id, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "Follow-up not found")
		return
	}
	w.WriteHeader(204)
}
