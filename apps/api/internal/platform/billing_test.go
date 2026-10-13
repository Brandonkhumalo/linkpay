package platform

import (
	"strings"
	"testing"
)

func TestPaynowHashOfficialVector(t *testing.T) {
	fields := []paynowField{
		{"status", "Ok"},
		{"browserurl", "https://staging.paynow.co.zw/Payment/ConfirmPayment/9510"},
		{"pollurl", "https://staging.paynow.co.zw/Interface/CheckPayment/?guid=c7ed41da-0159-46da-b428-69549f770413"},
		{"paynowreference", "9510"},
	}
	const key = "3e9fed89-60e1-4ce5-ab6e-6b1eb2d4f977"
	const want = "750DD0B0DF374678707BB5AF915AF81C228B9058AD57BB7120569EC68BBB9C2EFC1B26C6375D2BC562AC909B3CD6B2AF1D42E1A5E479FFAC8F4FB3FDCE71DF4D"
	if got := paynowHash(fields, key); got != want {
		t.Fatalf("hash = %s", got)
	}
	wire := encodePaynow(fields, key)
	parsed, err := parsePaynow(wire, key)
	if err != nil || parsed["paynowreference"] != "9510" {
		t.Fatalf("parse = %v, %v", parsed, err)
	}
	if _, err := parsePaynow(strings.Replace(wire, "9510", "9511", 1), key); err == nil {
		t.Fatal("tampered response accepted")
	}
	if _, err := parsePaynow(wire+"&status=Paid", key); err == nil {
		t.Fatal("duplicate status accepted")
	}
}

func TestPaynowURLRestriction(t *testing.T) {
	for _, raw := range []string{
		"http://www.paynow.co.zw/Interface/CheckPayment/?guid=a",
		"https://www.paynow.co.zw.evil.test/Interface/CheckPayment/?guid=a",
		"https://www.paynow.co.zw@evil.test/Interface/CheckPayment/?guid=a",
		"https://www.paynow.co.zw:443/Interface/CheckPayment/?guid=a",
		"https://www.paynow.co.zw/other?guid=a",
	} {
		if paynowURL(raw, false) {
			t.Errorf("accepted %s", raw)
		}
	}
	if !paynowURL("https://www.paynow.co.zw/Interface/CheckPayment/?guid=a", false) {
		t.Fatal("valid URL rejected")
	}
}

func TestBillingPrice(t *testing.T) {
	if billingPrice("starter", 1) != 300 || billingPrice("business", 12) != 6000 || billingPrice("free", 1) != 0 || billingPrice("starter", 13) != 0 {
		t.Fatal("incorrect price")
	}
	if invoiceTopupPrice(30) != 100 || invoiceTopupPrice(100) != 200 || invoiceTopupPrice(0) != 0 || invoiceTopupPrice(31) != 0 {
		t.Fatal("incorrect invoice top-up price")
	}
}
