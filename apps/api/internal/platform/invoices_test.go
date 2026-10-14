package platform

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-pdf/fpdf"
)

func TestValidInvoicePhone(t *testing.T) {
	for _, phone := range []string{"+263771234567", "+263 77 123 4567", "0771-234-567", "(263) 771 234 567"} {
		if !validInvoicePhone(phone) {
			t.Errorf("rejected valid phone %q", phone)
		}
	}
	for _, phone := range []string{"", "123456", "++263771234567", "263\n771234567", "call-me", "12345678901234567890123456789012345678901"} {
		if validInvoicePhone(phone) {
			t.Errorf("accepted invalid phone %q", phone)
		}
	}
}

func TestInvoiceDesignsRenderPDF(t *testing.T) {
	t.Setenv("MEDIA_DIR", t.TempDir())
	logoID := newID()
	logoPath := imagePath(logoID, "png")
	if err := os.MkdirAll(filepath.Dir(logoPath), 0700); err != nil {
		t.Fatal(err)
	}
	logoFile, err := os.Create(logoPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(logoFile, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		logoFile.Close()
		t.Fatal(err)
	}
	if err := logoFile.Close(); err != nil {
		t.Fatal(err)
	}
	for _, design := range []string{"simple", "modern", "corporate", "executive", "ledger"} {
		t.Run(design, func(t *testing.T) {
			if !validInvoiceDesign(design, "#235743") {
				t.Fatal("valid design rejected")
			}
			invoice := invoiceView{
				Number: 42, CustomerName: "Recipient Ltd", SenderBusinessName: "Sender Ltd", SenderPhone: "+263771234567",
				DesignTemplate: design, DesignColor: "#235743", Currency: "USD", DueAt: "2099-01-01", Status: "sent",
				ShowLogo: true, LogoID: logoID, LogoExt: "png",
				SubtotalMinor: 12500, TotalMinor: 12500, BalanceMinor: 12500,
				Items: []invoiceItem{{Description: "Consulting services", Quantity: 1, UnitMinor: 12500, TotalMinor: 12500}},
			}
			p := fpdf.New("P", "mm", "A4", "")
			p.SetMargins(18, 18, 18)
			p.AddPage()
			renderInvoicePDF(p, invoice, "Pay directly to Sender Ltd")
			var output bytes.Buffer
			if err := p.Output(&output); err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(output.Bytes(), []byte("%PDF-")) {
				t.Fatal("PDF output is missing")
			}
		})
	}
	for _, invalid := range [][2]string{{"unknown", "#235743"}, {"simple", "red"}, {"simple", "#12345g"}} {
		if validInvoiceDesign(invalid[0], invalid[1]) {
			t.Errorf("accepted invalid design %v", invalid)
		}
	}
}
