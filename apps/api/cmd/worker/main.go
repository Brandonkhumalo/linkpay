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
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err = a.RunReminders(ctx)
		cancel()
		if err != nil {
			log.Printf("reminder worker: %v", err)
		}
		billingCtx, billingCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if billingErr := a.ReconcileBilling(billingCtx); billingErr != nil {
			log.Printf("billing reconciliation: %v", billingErr)
		}
		billingCancel()
		time.Sleep(time.Hour)
	}
}
