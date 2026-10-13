package platform

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type reminderJob struct {
	ID, InvoiceID, BusinessID, Email, Customer, Name, Currency string
	Number, Total, Paid                                        int64
	Offset, Attempts                                           int
	Due                                                        time.Time
}

func (a *App) ScheduleReminders(ctx context.Context) error {
	rows, err := a.DB.Query(ctx, `SELECT i.id::text,i.business_id::text,i.due_at,rr.offsets,b.timezone
	FROM invoices i JOIN businesses b ON b.id=i.business_id JOIN reminder_rules rr ON rr.business_id=i.business_id
	WHERE i.status='sent' AND rr.enabled AND b.plan_expires_at>now() AND b.plan IN ('starter','business') AND i.due_at BETWEEN current_date-interval '60 days' AND current_date+interval '30 days'
	AND i.total_minor>COALESCE((SELECT sum(amount_minor) FROM payments WHERE business_id=i.business_id AND invoice_id=i.id),0)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var invoiceID, businessID, zone string
		var due time.Time
		var offsets []int32
		if err = rows.Scan(&invoiceID, &businessID, &due, &offsets, &zone); err != nil {
			return err
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			return err
		}
		for _, offset := range offsets {
			target := due.AddDate(0, 0, int(offset)).Format("2006-01-02")
			if target != time.Now().In(loc).Format("2006-01-02") {
				continue
			}
			_, err = a.DB.Exec(ctx, `INSERT INTO reminder_deliveries(id,business_id,invoice_id,offset_days,status) VALUES($1,$2,$3,$4,'pending') ON CONFLICT(invoice_id,offset_days) DO NOTHING`, newID(), businessID, invoiceID, offset)
			if err != nil {
				return err
			}
		}
	}
	return rows.Err()
}
func (a *App) claimReminder(ctx context.Context) (reminderJob, error) {
	var j reminderJob
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return j, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT d.id::text,d.invoice_id::text,d.business_id::text,d.offset_days,d.attempts,c.email,c.name,b.name,i.currency,i.number,i.total_minor,i.due_at,
	COALESCE((SELECT sum(amount_minor) FROM payments WHERE invoice_id=i.id AND business_id=i.business_id),0)
	FROM reminder_deliveries d JOIN invoices i ON i.id=d.invoice_id AND i.business_id=d.business_id
	JOIN customers c ON c.id=i.customer_id AND c.business_id=i.business_id JOIN businesses b ON b.id=i.business_id
	WHERE (d.status='pending' OR (d.status='sending' AND d.next_attempt_at<now())) AND d.next_attempt_at<=now()
	ORDER BY d.next_attempt_at LIMIT 1 FOR UPDATE OF d SKIP LOCKED`).Scan(&j.ID, &j.InvoiceID, &j.BusinessID, &j.Offset, &j.Attempts, &j.Email, &j.Customer, &j.Name, &j.Currency, &j.Number, &j.Total, &j.Due, &j.Paid)
	if err != nil {
		return j, err
	}
	_, err = tx.Exec(ctx, `UPDATE reminder_deliveries SET status='sending',attempts=attempts+1,next_attempt_at=now()+interval '10 minutes' WHERE id=$1`, j.ID)
	if err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}
func (a *App) RunReminders(ctx context.Context) error {
	if err := a.ScheduleReminders(ctx); err != nil {
		return err
	}
	for n := 0; n < 100; n++ {
		j, err := a.claimReminder(ctx)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		var active bool
		err = a.DB.QueryRow(ctx, `SELECT i.status='sent' AND rr.enabled AND $3=ANY(rr.offsets) AND b.plan_expires_at>now() AND b.plan IN ('starter','business') AND i.total_minor>COALESCE((SELECT sum(amount_minor) FROM payments WHERE invoice_id=i.id AND business_id=i.business_id),0)
		FROM invoices i JOIN reminder_rules rr ON rr.business_id=i.business_id JOIN businesses b ON b.id=i.business_id WHERE i.id=$1 AND i.business_id=$2`, j.InvoiceID, j.BusinessID, j.Offset).Scan(&active)
		if err != nil {
			return err
		}
		if !active || j.Email == "" {
			_, err = a.DB.Exec(ctx, `UPDATE reminder_deliveries SET status='skipped',last_error='not eligible' WHERE id=$1`, j.ID)
			if err != nil {
				return err
			}
			continue
		}
		var sentThisMonth int
		if err = a.DB.QueryRow(ctx, `SELECT count(*) FROM reminder_deliveries WHERE business_id=$1 AND status='sent' AND sent_at>=date_trunc('month',now())`, j.BusinessID).Scan(&sentThisMonth); err != nil {
			return err
		}
		var plan string
		if err = a.DB.QueryRow(ctx, `SELECT plan FROM businesses WHERE id=$1`, j.BusinessID).Scan(&plan); err != nil {
			return err
		}
		limit := 100
		if plan == "business" {
			limit = 500
		}
		if sentThisMonth >= limit {
			_, err = a.DB.Exec(ctx, `UPDATE reminder_deliveries SET status='failed',last_error='monthly email allowance reached' WHERE id=$1`, j.ID)
			if err != nil {
				return err
			}
			continue
		}
		err = a.sendReminder(j)
		if err == nil {
			_, err = a.DB.Exec(ctx, `UPDATE reminder_deliveries SET status='sent',sent_at=now(),last_error='' WHERE id=$1`, j.ID)
		} else if j.Attempts >= 3 {
			_, err = a.DB.Exec(ctx, `UPDATE reminder_deliveries SET status='failed',last_error='delivery failed' WHERE id=$1`, j.ID)
		} else {
			_, err = a.DB.Exec(ctx, `UPDATE reminder_deliveries SET status='pending',next_attempt_at=now()+interval '1 hour',last_error='delivery failed' WHERE id=$1`, j.ID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (a *App) sendReminder(j reminderJob) error {
	subject := fmt.Sprintf("Invoice %06d payment reminder", j.Number)
	link := a.PublicURL + "/invoice/" + a.invoiceToken(j.InvoiceID)
	body := fmt.Sprintf("Hello %s,\r\n\r\nThis is a reminder from %s for invoice %06d.\r\nDue date: %s\r\nOutstanding: %s %s\r\nView invoice: %s\r\n\r\nPlease contact the business directly if you have already paid.\r\n", j.Customer, j.Name, j.Number, j.Due.Format("2006-01-02"), j.Currency, money(j.Total-j.Paid), link)
	return sendPlainEmail(j.Email, subject, body)
}
func sendPlainEmail(recipient, subject, body string) error {
	host := os.Getenv("SMTP_HOST")
	from := os.Getenv("SMTP_FROM")
	user := os.Getenv("SMTP_USER")
	password := os.Getenv("SMTP_PASSWORD")
	if host == "" || from == "" || user == "" || password == "" {
		return errors.New("SMTP is not configured")
	}
	fromAddress, err := mail.ParseAddress(from)
	if err != nil || fromAddress.Address != from || strings.ContainsAny(from, "\r\n") {
		return errors.New("invalid SMTP_FROM")
	}
	toAddress, err := mail.ParseAddress(recipient)
	if err != nil || toAddress.Address != recipient || strings.ContainsAny(recipient, "\r\n") {
		return errors.New("invalid recipient")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return errors.New("invalid subject")
	}
	port := env("SMTP_PORT", "587")
	address := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(25 * time.Second)); err != nil {
		return err
	}
	var client *smtp.Client
	if port == "465" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err = tlsConn.Handshake(); err != nil {
			return err
		}
		client, err = smtp.NewClient(tlsConn, host)
	} else {
		client, err = smtp.NewClient(conn, host)
		if err == nil {
			err = client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		}
	}
	if err != nil {
		return err
	}
	defer client.Close()
	if err = client.Auth(smtp.PlainAuth("", user, password, host)); err != nil {
		return err
	}
	if err = client.Mail(from); err != nil {
		return err
	}
	if err = client.Rcpt(recipient); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	msg := "From: " + from + "\r\nTo: " + recipient + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body
	if _, err = w.Write([]byte(msg)); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
func ParseSMTPPort() error {
	port := env("SMTP_PORT", "587")
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("invalid SMTP_PORT")
	}
	return nil
}
