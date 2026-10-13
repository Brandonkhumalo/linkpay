package main

import (
	"context"
	"log"
	"os"
	"time"

	"linkpay/api/internal/platform"
)

func main() {
	if err := platform.ParseSMTPPort(); err != nil {
		log.Fatal(err)
	}
	a, err := platform.New(context.Background(), os.Getenv("DATABASE_URL"), os.Getenv("PUBLIC_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	smtpConfigured := os.Getenv("SMTP_HOST") != "" && os.Getenv("SMTP_USER") != "" && os.Getenv("SMTP_PASSWORD") != "" && os.Getenv("SMTP_FROM") != ""
	if !smtpConfigured {
		log.Print("SMTP is not configured; email reminders are disabled")
	}
	for {
		followupCtx, followupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if followupErr := a.RunFollowupAlerts(followupCtx); followupErr != nil {
			log.Printf("follow-up alerts: %v", followupErr)
		}
		followupCancel()
		if smtpConfigured {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			err = a.RunReminders(ctx)
			cancel()
			if err != nil {
				log.Printf("reminder worker: %v", err)
			}
			notificationCtx, notificationCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			if notificationErr := a.RunNotifications(notificationCtx); notificationErr != nil {
				log.Printf("merchant notifications: %v", notificationErr)
			}
			notificationCancel()
		}
		billingCtx, billingCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if billingErr := a.ReconcileBilling(billingCtx); billingErr != nil {
			log.Printf("billing reconciliation: %v", billingErr)
		}
		billingCancel()
		draftCtx, draftCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if draftErr := a.RunRecurringDrafts(draftCtx); draftErr != nil {
			log.Printf("recurring drafts: %v", draftErr)
		}
		draftCancel()
		time.Sleep(time.Hour)
	}
}
