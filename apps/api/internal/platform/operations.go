package platform

import (
	"net/http"
	"sort"
	"time"
)

type reminderSettings struct {
	Enabled bool    `json:"enabled"`
	Offsets []int32 `json:"offsets"`
}

func (a *App) reminderRules(w http.ResponseWriter, r *http.Request) {
	var s reminderSettings
	err := a.DB.QueryRow(r.Context(), `SELECT enabled,offsets FROM reminder_rules WHERE business_id=$1`, who(r).BusinessID).Scan(&s.Enabled, &s.Offsets)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, s)
}
func (a *App) updateReminderRules(w http.ResponseWriter, r *http.Request) {
	var s reminderSettings
	if !decode(w, r, &s) {
		return
	}
	if len(s.Offsets) > 8 {
		fail(w, 400, "Too many reminder days")
		return
	}
	seen := map[int32]bool{}
	for _, n := range s.Offsets {
		if n < -30 || n > 60 || seen[n] {
			fail(w, 400, "Invalid reminder schedule")
			return
		}
		seen[n] = true
	}
	sort.Slice(s.Offsets, func(i, j int) bool { return s.Offsets[i] < s.Offsets[j] })
	var plan string
	err := a.DB.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END FROM businesses WHERE id=$1`, who(r).BusinessID).Scan(&plan)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if s.Enabled && plan == "free" {
		fail(w, 403, "Automated reminders require a paid plan")
		return
	}
	_, err = a.DB.Exec(r.Context(), `UPDATE reminder_rules SET enabled=$1,offsets=$2,updated_at=now() WHERE business_id=$3`, s.Enabled, s.Offsets, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, s)
}
func (a *App) reminderDeliveries(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT invoice_id::text,offset_days,status,attempts,COALESCE(sent_at::text,'') FROM reminder_deliveries WHERE business_id=$1 ORDER BY next_attempt_at DESC,id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, status, sent string
		var offset, attempts int
		if err = rows.Scan(&id, &offset, &status, &attempts, &sent); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, map[string]any{"invoice_id": id, "offset_days": offset, "status": status, "attempts": attempts, "sent_at": sent})
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}
func (a *App) overview(w http.ResponseWriter, r *http.Request) {
	var invoiced, received, outstanding, overdue int64
	var invoiceCount, customerCount, enquiryCount int
	err := a.DB.QueryRow(r.Context(), `SELECT COALESCE(sum(total_minor),0),count(*) FROM invoices WHERE business_id=$1 AND status='sent' AND currency=(SELECT currency FROM businesses WHERE id=$1)`, who(r).BusinessID).Scan(&invoiced, &invoiceCount)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	err = a.DB.QueryRow(r.Context(), `SELECT COALESCE(sum(p.amount_minor),0) FROM payments p JOIN invoices i ON i.id=p.invoice_id AND i.business_id=p.business_id WHERE p.business_id=$1 AND i.currency=(SELECT currency FROM businesses WHERE id=$1)`, who(r).BusinessID).Scan(&received)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	outstanding = invoiced - received
	err = a.DB.QueryRow(r.Context(), `SELECT COALESCE(sum(i.total_minor-COALESCE(p.paid,0)),0) FROM invoices i JOIN businesses b ON b.id=i.business_id LEFT JOIN LATERAL (SELECT sum(amount_minor) paid FROM payments WHERE invoice_id=i.id AND business_id=i.business_id) p ON true WHERE i.business_id=$1 AND i.status='sent' AND i.due_at<timezone(b.timezone,now())::date AND i.total_minor>COALESCE(p.paid,0)`, who(r).BusinessID).Scan(&overdue)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = a.DB.QueryRow(r.Context(), `SELECT count(*) FROM customers WHERE business_id=$1`, who(r).BusinessID).Scan(&customerCount); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = a.DB.QueryRow(r.Context(), `SELECT count(*) FROM enquiries WHERE business_id=$1`, who(r).BusinessID).Scan(&enquiryCount); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"invoiced_minor": invoiced, "received_minor": received, "outstanding_minor": outstanding, "overdue_minor": overdue, "invoice_count": invoiceCount, "customer_count": customerCount, "enquiry_count": enquiryCount})
}
func (a *App) subscription(w http.ResponseWriter, r *http.Request) {
	var plan string
	var products, monthly int
	var expires *time.Time
	var trialPlan *string
	var trialStarted, trialExpires *time.Time
	var trialActive bool
	err := a.DB.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,plan_expires_at,trial_plan,trial_started_at,trial_expires_at,COALESCE(trial_expires_at>now() AND plan=trial_plan AND plan_expires_at=trial_expires_at,false) FROM businesses WHERE id=$1`, who(r).BusinessID).Scan(&plan, &expires, &trialPlan, &trialStarted, &trialExpires, &trialActive)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = a.DB.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE business_id=$1`, who(r).BusinessID).Scan(&products); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = a.DB.QueryRow(r.Context(), `SELECT count(*) FROM invoices WHERE business_id=$1 AND created_at>=date_trunc('month',now())`, who(r).BusinessID).Scan(&monthly); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"plan": plan, "expires_at": expires, "trial_plan": trialPlan, "trial_started_at": trialStarted, "trial_expires_at": trialExpires, "trial_active": trialActive, "product_limit": planProductLimit(plan), "invoice_monthly_limit": planInvoiceLimit(plan), "products_used": products, "invoices_used": monthly, "period_start": time.Now().UTC().Format("2006-01")})
}
