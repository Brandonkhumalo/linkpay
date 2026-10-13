package platform

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistrationPayloadMatchesWebForm(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(`{"email":"owner@example.test","password":"long-enough-password","business_name":"Example Shop","slug":"example-shop","trial_plan":"starter"}`))
	w := httptest.NewRecorder()
	var input registerInput
	if !decode(w, r, &input) || input.BusinessName != "Example Shop" || input.TrialPlan != "starter" {
		t.Fatalf("registration form rejected: status=%d input=%+v", w.Code, input)
	}
}

func TestInvoiceStatus(t *testing.T) {
	tests := []struct {
		name, issued, due, today, want string
		void                           bool
		paid, total                    int64
	}{
		{"draft", "", "2026-10-10", "2026-10-12", "draft", false, 0, 1000},
		{"sent", "2026-10-01", "2026-10-12", "2026-10-12", "sent", false, 0, 1000},
		{"partial", "2026-10-01", "2026-10-12", "2026-10-12", "partially_paid", false, 400, 1000},
		{"overdue partial", "2026-10-01", "2026-10-11", "2026-10-12", "overdue", false, 400, 1000},
		{"paid", "2026-10-01", "2026-10-11", "2026-10-12", "paid", false, 1000, 1000},
		{"void", "2026-10-01", "2026-10-11", "2026-10-12", "void", true, 0, 1000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := invoiceStatus(tc.issued, tc.void, tc.paid, tc.total, tc.due, tc.today)
			if got != tc.want {
				t.Fatalf("status = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTOTPKnownVector(t *testing.T) {
	if got := totpCode([]byte("12345678901234567890"), 59/30); got != "287082" {
		t.Fatalf("TOTP = %s, want 287082", got)
	}
}

func TestPlanLimits(t *testing.T) {
	for _, tc := range []struct {
		plan               string
		products, invoices int
	}{
		{"free", 5, 30}, {"starter", 30, 60}, {"business", 500, 500},
	} {
		if got := planProductLimit(tc.plan); got != tc.products {
			t.Errorf("%s product limit = %d", tc.plan, got)
		}
		if got := planInvoiceLimit(tc.plan); got != tc.invoices {
			t.Errorf("%s invoice limit = %d", tc.plan, got)
		}
	}
}
