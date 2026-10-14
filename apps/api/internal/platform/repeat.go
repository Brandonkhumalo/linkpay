package platform

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var errInvoiceQuota = errors.New("invoice quota exhausted")
var errInvoiceSenderPhone = errors.New("invoice sender phone is missing or invalid")
var errInvoiceLogoUnavailable = errors.New("invoice logo is unavailable")

func (a *App) insertDraftInvoice(ctx context.Context, tx pgx.Tx, businessID, actorID string, in invoiceInput, sourceQuoteID string) (string, int64, error) {
	var plan, currency, senderBusinessName, profilePhone, businessColor string
	var currentLogoID, currentLogoExt *string
	var currentLogoSize int64
	var number int64
	if err := tx.QueryRow(ctx, `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,currency,next_invoice_number,name,whatsapp,theme_color,logo_id::text,logo_ext,logo_size FROM businesses WHERE id=$1 FOR UPDATE`, businessID).Scan(&plan, &currency, &number, &senderBusinessName, &profilePhone, &businessColor, &currentLogoID, &currentLogoExt, &currentLogoSize); err != nil {
		return "", 0, err
	}
	if in.DesignTemplate == "" {
		in.DesignTemplate = "simple"
	}
	if in.DesignColor == "" {
		in.DesignColor = businessColor
	}
	if !validInvoiceDesign(in.DesignTemplate, in.DesignColor) {
		return "", 0, errors.New("invalid invoice design")
	}
	if in.ShowLogo && in.LogoID == "" {
		if currentLogoID == nil || currentLogoExt == nil {
			return "", 0, errInvoiceLogoUnavailable
		}
		in.LogoID, in.LogoExt, in.LogoSize = *currentLogoID, *currentLogoExt, currentLogoSize
	}
	monthly, topups, err := invoiceQuota(ctx, tx, businessID)
	if err != nil {
		return "", 0, err
	}
	quotaSource := "subscription"
	if monthly >= planInvoiceLimit(plan) {
		if topups <= 0 {
			return "", 0, errInvoiceQuota
		}
		quotaSource = "invoice_topup"
	}
	var customerName string
	if err = tx.QueryRow(ctx, `SELECT name FROM customers WHERE id=$1 AND business_id=$2`, in.CustomerID, businessID).Scan(&customerName); err != nil {
		return "", 0, err
	}
	if in.RecipientBusinessName == "" {
		in.RecipientBusinessName = customerName
	}
	if in.SenderPhone == "" {
		in.SenderPhone = profilePhone
	}
	if !validInvoicePhone(in.SenderPhone) {
		return "", 0, errInvoiceSenderPhone
	}
	var subtotal int64
	for i := range in.Items {
		in.Items[i].TotalMinor = in.Items[i].Quantity * in.Items[i].UnitMinor
		subtotal += in.Items[i].TotalMinor
	}
	var source any
	if sourceQuoteID != "" {
		source = sourceQuoteID
	}
	id := newID()
	var logoID, logoExt any
	if in.ShowLogo {
		logoID, logoExt = in.LogoID, in.LogoExt
	}
	_, err = tx.Exec(ctx, `INSERT INTO invoices(id,business_id,customer_id,number,currency,due_at,subtotal_minor,discount_minor,tax_minor,total_minor,notes,quota_source,source_quote_id,recipient_business_name,recipient_email,recipient_phone,sender_business_name,sender_phone,design_template,design_color,show_logo,logo_id,logo_ext,logo_size) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`, id, businessID, in.CustomerID, number, currency, in.DueAt, subtotal, in.DiscountMinor, in.TaxMinor, subtotal-in.DiscountMinor+in.TaxMinor, in.Notes, quotaSource, source, in.RecipientBusinessName, in.RecipientEmail, in.RecipientPhone, senderBusinessName, in.SenderPhone, in.DesignTemplate, in.DesignColor, in.ShowLogo, logoID, logoExt, in.LogoSize)
	if err != nil {
		return "", 0, err
	}
	for _, it := range in.Items {
		if _, err = tx.Exec(ctx, `INSERT INTO invoice_items(id,business_id,invoice_id,description,quantity,unit_minor,total_minor) VALUES($1,$2,$3,$4,$5,$6,$7)`, newID(), businessID, id, it.Description, it.Quantity, it.UnitMinor, it.TotalMinor); err != nil {
			return "", 0, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE businesses SET next_invoice_number=next_invoice_number+1 WHERE id=$1`, businessID); err != nil {
		return "", 0, err
	}
	var actor any
	if actorID != "" {
		actor = actorID
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(id,business_id,actor_id,action,subject_id) VALUES($1,$2,$3,'invoice.created',$4)`, newID(), businessID, actor, id); err != nil {
		return "", 0, err
	}
	return id, number, nil
}

func loadTemplate(ctx context.Context, tx pgx.Tx, businessID, invoiceID string) (invoiceInput, error) {
	var in invoiceInput
	err := tx.QueryRow(ctx, `SELECT i.customer_id::text,i.notes,i.discount_minor,i.tax_minor,COALESCE(i.recipient_business_name,c.name),COALESCE(i.recipient_email,c.email),COALESCE(i.recipient_phone,c.phone),COALESCE(i.sender_phone,b.whatsapp),i.design_template,i.design_color,i.show_logo,COALESCE(i.logo_id::text,''),COALESCE(i.logo_ext,''),i.logo_size FROM invoices i JOIN customers c ON c.id=i.customer_id AND c.business_id=i.business_id JOIN businesses b ON b.id=i.business_id WHERE i.id=$1 AND i.business_id=$2 AND i.status!='void'`, invoiceID, businessID).Scan(&in.CustomerID, &in.Notes, &in.DiscountMinor, &in.TaxMinor, &in.RecipientBusinessName, &in.RecipientEmail, &in.RecipientPhone, &in.SenderPhone, &in.DesignTemplate, &in.DesignColor, &in.ShowLogo, &in.LogoID, &in.LogoExt, &in.LogoSize)
	if err != nil {
		return in, err
	}
	rows, err := tx.Query(ctx, `SELECT description,quantity,unit_minor FROM invoice_items WHERE invoice_id=$1 AND business_id=$2 ORDER BY id`, invoiceID, businessID)
	if err != nil {
		return in, err
	}
	defer rows.Close()
	for rows.Next() {
		var item invoiceItem
		if err = rows.Scan(&item.Description, &item.Quantity, &item.UnitMinor); err != nil {
			return in, err
		}
		in.Items = append(in.Items, item)
	}
	return in, rows.Err()
}

func (a *App) duplicateInvoice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice not found")
		return
	}
	var in struct {
		DueAt string `json:"due_at"`
	}
	if !decode(w, r, &in) {
		return
	}
	due, err := time.Parse("2006-01-02", in.DueAt)
	if err != nil || due.Before(time.Now().UTC().AddDate(0, 0, -1)) {
		fail(w, 400, "Choose a valid due date")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	template, err := loadTemplate(r.Context(), tx, who(r).BusinessID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Invoice not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	template.DueAt = in.DueAt
	newID, number, err := a.insertDraftInvoice(r.Context(), tx, who(r).BusinessID, who(r).UserID, template, "")
	if errors.Is(err, errInvoiceQuota) {
		fail(w, 403, "Monthly invoice limit reached. Buy an invoice top-up to continue.")
		return
	}
	if errors.Is(err, errInvoiceSenderPhone) {
		fail(w, 400, "Add an international sender phone number in your storefront settings before duplicating this invoice")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": newID, "number": number})
}

func (a *App) convertQuote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Quote not found")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var enquiryID, description, status, zone string
	var amount int64
	err = tx.QueryRow(r.Context(), `SELECT q.enquiry_id::text,q.description,q.amount_minor,q.status,b.timezone FROM quotes q JOIN businesses b ON b.id=q.business_id WHERE q.id=$1 AND q.business_id=$2 FOR UPDATE OF q`, id, who(r).BusinessID).Scan(&enquiryID, &description, &amount, &status, &zone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Quote not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if status != "accepted" {
		fail(w, 409, "Only accepted quotes can become invoices")
		return
	}
	var name, email, phone string
	if err = tx.QueryRow(r.Context(), `SELECT COALESCE(NULLIF(brand_name,''),name),email,phone FROM enquiries WHERE id=$1 AND business_id=$2`, enquiryID, who(r).BusinessID).Scan(&name, &email, &phone); err != nil {
		unavailable(w, r, err)
		return
	}
	var customerID string
	err = tx.QueryRow(r.Context(), `SELECT id::text FROM customers WHERE business_id=$1 AND (($2<>'' AND lower(email)=lower($2)) OR ($3<>'' AND phone=$3)) ORDER BY created_at LIMIT 1`, who(r).BusinessID, email, phone).Scan(&customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		customerID = newID()
		if _, err = tx.Exec(r.Context(), `INSERT INTO customers(id,business_id,name,email,phone,notes) VALUES($1,$2,$3,$4,$5,'Created from accepted quote')`, customerID, who(r).BusinessID, name, email, phone); err != nil {
			unavailable(w, r, err)
			return
		}
	} else if err != nil {
		unavailable(w, r, err)
		return
	}
	summary := []rune(description)
	if len(description) > 500 {
		for len(string(summary)) > 497 {
			summary = summary[:len(summary)-1]
		}
		summary = append(summary, '…')
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	in := invoiceInput{CustomerID: customerID, RecipientBusinessName: name, RecipientEmail: email, RecipientPhone: phone, DueAt: time.Now().In(location).AddDate(0, 0, 14).Format("2006-01-02"), Notes: description, Items: []invoiceItem{{Description: string(summary), Quantity: 1, UnitMinor: amount}}}
	invoiceID, number, err := a.insertDraftInvoice(r.Context(), tx, who(r).BusinessID, who(r).UserID, in, id)
	if errors.Is(err, errInvoiceQuota) {
		fail(w, 403, "Monthly invoice limit reached. Buy an invoice top-up to continue.")
		return
	}
	if errors.Is(err, errInvoiceSenderPhone) {
		fail(w, 400, "Add an international sender phone number in your storefront settings before converting this quote")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE quotes SET status='converted',invoice_id=$1 WHERE id=$2 AND business_id=$3`, invoiceID, id, who(r).BusinessID); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": invoiceID, "number": number})
}

func (a *App) updateInvoiceSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice not found")
		return
	}
	var in struct {
		IntervalMonths int    `json:"interval_months"`
		NextRunOn      string `json:"next_run_on"`
		Active         bool   `json:"active"`
	}
	if !decode(w, r, &in) {
		return
	}
	date, err := time.Parse("2006-01-02", in.NextRunOn)
	if in.IntervalMonths < 1 || in.IntervalMonths > 12 || err != nil || date.Before(time.Now().UTC().Truncate(24*time.Hour)) || date.After(time.Now().UTC().AddDate(1, 0, 0)) {
		fail(w, 400, "Invalid recurring draft schedule")
		return
	}
	var exists bool
	if err = a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM invoices WHERE id=$1 AND business_id=$2 AND status!='void')`, id, who(r).BusinessID).Scan(&exists); err != nil {
		unavailable(w, r, err)
		return
	}
	if !exists {
		fail(w, 404, "Invoice not found")
		return
	}
	_, err = a.DB.Exec(r.Context(), `INSERT INTO invoice_schedules(id,business_id,source_invoice_id,interval_months,next_run_on,active) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(business_id,source_invoice_id) DO UPDATE SET interval_months=EXCLUDED.interval_months,next_run_on=EXCLUDED.next_run_on,active=EXCLUDED.active,last_error=''`, newID(), who(r).BusinessID, id, in.IntervalMonths, in.NextRunOn, in.Active)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"active": in.Active})
}

func (a *App) invoiceSchedules(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,source_invoice_id::text,interval_months,next_run_on::text,active,last_error FROM invoice_schedules WHERE business_id=$1 ORDER BY next_run_on,id LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, source, next, last string
		var interval int
		var active bool
		if err = rows.Scan(&id, &source, &interval, &next, &active, &last); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, map[string]any{"id": id, "source_invoice_id": source, "interval_months": interval, "next_run_on": next, "active": active, "last_error": last})
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}

func (a *App) invoiceSchedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Invoice not found")
		return
	}
	var exists bool
	if err := a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM invoices WHERE id=$1 AND business_id=$2)`, id, who(r).BusinessID).Scan(&exists); err != nil {
		unavailable(w, r, err)
		return
	}
	if !exists {
		fail(w, 404, "Invoice not found")
		return
	}
	var scheduleID, next, last string
	var interval int
	var active bool
	err := a.DB.QueryRow(r.Context(), `SELECT id::text,interval_months,next_run_on::text,active,last_error FROM invoice_schedules WHERE source_invoice_id=$1 AND business_id=$2`, id, who(r).BusinessID).Scan(&scheduleID, &interval, &next, &active, &last)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 200, map[string]any{})
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"id": scheduleID, "interval_months": interval, "next_run_on": next, "active": active, "last_error": last})
}

func (a *App) RunRecurringDrafts(ctx context.Context) error {
	for n := 0; n < 50; n++ {
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			return err
		}
		var scheduleID, businessID, sourceID, zone string
		var interval int
		var runOn time.Time
		err = tx.QueryRow(ctx, `SELECT s.id::text,s.business_id::text,s.source_invoice_id::text,s.interval_months,s.next_run_on,b.timezone FROM invoice_schedules s JOIN businesses b ON b.id=s.business_id WHERE s.active AND s.next_run_on<=timezone(b.timezone,now())::date ORDER BY s.next_run_on FOR UPDATE OF s SKIP LOCKED LIMIT 1`).Scan(&scheduleID, &businessID, &sourceID, &interval, &runOn, &zone)
		if errors.Is(err, pgx.ErrNoRows) {
			tx.Rollback(ctx)
			return nil
		}
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		in, loadErr := loadTemplate(ctx, tx, businessID, sourceID)
		if errors.Is(loadErr, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `UPDATE invoice_schedules SET active=false,last_error='Source invoice is unavailable' WHERE id=$1`, scheduleID)
			if err != nil {
				tx.Rollback(ctx)
				return err
			}
			if err = tx.Commit(ctx); err != nil {
				return err
			}
			continue
		}
		if loadErr != nil {
			tx.Rollback(ctx)
			return loadErr
		}
		location, loadErr := time.LoadLocation(zone)
		if loadErr != nil {
			tx.Rollback(ctx)
			return loadErr
		}
		in.DueAt = time.Now().In(location).AddDate(0, 0, 14).Format("2006-01-02")
		_, _, err = a.insertDraftInvoice(ctx, tx, businessID, "", in, "")
		if errors.Is(err, errInvoiceQuota) {
			_, err = tx.Exec(ctx, `UPDATE invoice_schedules SET last_error='Monthly invoice limit reached' WHERE id=$1`, scheduleID)
			if err != nil {
				tx.Rollback(ctx)
				return err
			}
			if err = tx.Commit(ctx); err != nil {
				return err
			}
			return nil
		}
		if errors.Is(err, errInvoiceSenderPhone) {
			_, err = tx.Exec(ctx, `UPDATE invoice_schedules SET last_error='Add an international phone number to your storefront before generating the next invoice' WHERE id=$1`, scheduleID)
			if err != nil {
				tx.Rollback(ctx)
				return err
			}
			return tx.Commit(ctx)
		}
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE invoice_schedules SET next_run_on=GREATEST(($2::date + make_interval(months=>$3))::date,(timezone($4,now())::date + make_interval(months=>$3))::date),last_error='' WHERE id=$1`, scheduleID, runOn, interval, zone)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
