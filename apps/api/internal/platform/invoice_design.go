package platform

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

func validInvoiceDesign(template, color string) bool {
	switch template {
	case "simple", "modern", "corporate", "executive", "ledger":
	default:
		return false
	}
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	_, err := hex.DecodeString(color[1:])
	return err == nil
}

func renderInvoicePDF(p *fpdf.Fpdf, invoice invoiceView, paymentInstructions string) {
	accent, _ := hex.DecodeString(invoice.DesignColor[1:])
	r, g, b := int(accent[0]), int(accent[1]), int(accent[2])
	textOnAccent := []int{255, 255, 255}
	if r*299+g*587+b*114 > 160000 {
		textOnAccent = []int{26, 39, 35}
	}
	setAccent := func() { p.SetTextColor(r, g, b) }
	setBody := func() { p.SetTextColor(35, 48, 44) }
	drawLogo := func(x, y float64) {
		if !invoice.ShowLogo || invoice.LogoID == "" {
			return
		}
		p.SetFillColor(255, 255, 255)
		p.RoundedRect(x, y, 27, 27, 2, "1234", "F")
		p.ImageOptions(imagePath(invoice.LogoID, invoice.LogoExt), x+1, y+1, 25, 25, false, fpdf.ImageOptions{ImageType: strings.ToUpper(invoice.LogoExt)}, 0, "")
	}
	setBody()
	var metaY float64
	switch invoice.DesignTemplate {
	case "modern":
		p.SetFillColor(r, g, b)
		p.Rect(0, 0, 210, 66, "F")
		drawLogo(18, 17)
		p.SetTextColor(textOnAccent[0], textOnAccent[1], textOnAccent[2])
		p.SetFont("Arial", "B", 19)
		x := 18.0
		if invoice.ShowLogo {
			x = 52
		}
		p.SetXY(x, 21)
		p.Cell(125, 10, invoice.SenderBusinessName)
		p.SetFont("Arial", "", 10)
		p.SetXY(x, 33)
		p.Cell(125, 6, invoice.SenderPhone)
		p.SetFont("Arial", "B", 16)
		p.SetXY(18, 48)
		p.Cell(175, 10, fmt.Sprintf("INVOICE  #%06d", invoice.Number))
		metaY = 78
	case "corporate":
		p.SetFillColor(r, g, b)
		p.Rect(18, 18, 3, 42, "F")
		drawLogo(27, 20)
		x := 28.0
		if invoice.ShowLogo {
			x = 59
		}
		p.SetFont("Arial", "B", 16)
		p.SetXY(x, 21)
		p.Cell(85, 9, invoice.SenderBusinessName)
		p.SetFont("Arial", "", 10)
		p.SetXY(x, 33)
		p.Cell(85, 7, invoice.SenderPhone)
		setAccent()
		p.SetFont("Arial", "B", 23)
		p.SetXY(136, 19)
		p.Cell(55, 12, "INVOICE")
		setBody()
		p.SetFont("Arial", "", 10)
		p.SetXY(149, 33)
		p.Cell(43, 7, fmt.Sprintf("#%06d", invoice.Number))
		p.SetDrawColor(r, g, b)
		p.Line(18, 64, 192, 64)
		metaY = 75
	case "executive":
		p.SetFillColor(25, 37, 42)
		p.Rect(0, 0, 210, 70, "F")
		p.SetFillColor(r, g, b)
		p.Rect(0, 67, 210, 3, "F")
		drawLogo(18, 19)
		p.SetTextColor(255, 255, 255)
		p.SetFont("Arial", "B", 24)
		p.SetXY(18, 19)
		p.CellFormat(174, 12, "INVOICE", "", 0, "C", false, 0, "")
		p.SetFont("Arial", "", 11)
		p.SetXY(18, 34)
		p.CellFormat(174, 8, invoice.SenderBusinessName+"  /  "+invoice.SenderPhone, "", 0, "C", false, 0, "")
		p.SetXY(18, 49)
		p.CellFormat(174, 7, fmt.Sprintf("REFERENCE  %06d", invoice.Number), "", 0, "C", false, 0, "")
		metaY = 82
	case "ledger":
		p.SetDrawColor(r, g, b)
		p.SetLineWidth(0.7)
		p.Line(18, 18, 192, 18)
		p.Rect(18, 24, 174, 39, "D")
		drawLogo(159, 29)
		p.SetFont("Arial", "B", 15)
		p.SetXY(25, 28)
		p.Cell(126, 8, invoice.SenderBusinessName)
		p.SetFont("Arial", "", 10)
		p.SetXY(25, 39)
		p.Cell(126, 6, invoice.SenderPhone)
		setAccent()
		p.SetFont("Arial", "B", 10)
		p.SetXY(25, 51)
		p.Cell(126, 7, fmt.Sprintf("INVOICE REFERENCE  %06d", invoice.Number))
		metaY = 74
	default:
		drawLogo(18, 18)
		x := 18.0
		if invoice.ShowLogo {
			x = 52
		}
		p.SetFont("Arial", "B", 17)
		p.SetXY(x, 20)
		p.Cell(126, 10, invoice.SenderBusinessName)
		p.SetFont("Arial", "", 10)
		p.SetXY(x, 32)
		p.Cell(126, 7, invoice.SenderPhone)
		setAccent()
		p.SetFont("Arial", "B", 21)
		p.SetXY(18, 51)
		p.Cell(174, 12, fmt.Sprintf("INVOICE #%06d", invoice.Number))
		p.SetDrawColor(r, g, b)
		p.Line(18, 68, 192, 68)
		metaY = 77
	}
	setBody()
	if invoice.DesignTemplate == "modern" || invoice.DesignTemplate == "ledger" {
		p.SetFillColor(244, 247, 246)
		p.Rect(18, metaY-2, 174, 40, "F")
	}
	p.SetFont("Arial", "B", 9)
	p.SetXY(22, metaY)
	p.Cell(85, 6, "BILL TO")
	p.SetXY(116, metaY)
	p.Cell(72, 6, "INVOICE DETAILS")
	p.SetFont("Arial", "B", 11)
	p.SetXY(22, metaY+7)
	p.MultiCell(87, 6, invoice.CustomerName, "", "L", false)
	leftY := p.GetY()
	p.SetFont("Arial", "", 9)
	if invoice.CustomerEmail != "" {
		p.SetXY(22, leftY)
		p.Cell(87, 5, invoice.CustomerEmail)
		leftY += 5
	}
	if invoice.CustomerPhone != "" {
		p.SetXY(22, leftY)
		p.Cell(87, 5, invoice.CustomerPhone)
		leftY += 5
	}
	p.SetXY(116, metaY+7)
	p.Cell(72, 6, fmt.Sprintf("Number: %06d", invoice.Number))
	p.SetXY(116, metaY+14)
	p.Cell(72, 6, "Due: "+invoice.DueAt)
	p.SetXY(116, metaY+21)
	p.Cell(72, 6, "Status: "+strings.ToUpper(invoice.Status))
	bottom := metaY + 39
	if leftY+4 > bottom {
		bottom = leftY + 4
	}
	p.SetY(bottom + 7)
	invoiceTableHeader(p, invoice.DesignTemplate, r, g, b, textOnAccent)
	p.SetFont("Arial", "", 10)
	for index, item := range invoice.Items {
		lines := p.SplitLines([]byte(item.Description), 103)
		height := float64(len(lines))*5 + 4
		if height < 9 {
			height = 9
		}
		if p.GetY()+height > 273 {
			p.AddPage()
			p.SetY(20)
			invoiceTableHeader(p, invoice.DesignTemplate, r, g, b, textOnAccent)
			p.SetFont("Arial", "", 10)
		}
		y := p.GetY()
		if (invoice.DesignTemplate == "modern" || invoice.DesignTemplate == "executive") && index%2 == 0 {
			p.SetFillColor(246, 249, 248)
			p.Rect(18, y, 174, height, "F")
		}
		p.SetXY(20, y+2)
		p.MultiCell(103, 5, item.Description, "", "L", false)
		p.SetXY(127, y+2)
		p.CellFormat(23, 5, fmt.Sprint(item.Quantity), "", 0, "R", false, 0, "")
		p.SetXY(151, y+2)
		p.CellFormat(39, 5, invoice.Currency+" "+money(item.TotalMinor), "", 0, "R", false, 0, "")
		p.SetDrawColor(221, 227, 224)
		p.Line(18, y+height, 192, y+height)
		p.SetY(y + height)
	}
	if p.GetY() > 221 {
		p.AddPage()
		p.SetY(24)
	}
	p.Ln(9)
	for _, line := range []struct {
		label  string
		amount int64
	}{
		{"Subtotal", invoice.SubtotalMinor},
		{"Discount", -invoice.DiscountMinor},
		{"Tax", invoice.TaxMinor},
		{"Paid", -invoice.PaidMinor},
	} {
		p.SetFont("Arial", "", 10)
		p.SetXY(109, p.GetY())
		p.CellFormat(44, 7, line.label, "", 0, "L", false, 0, "")
		amount := line.amount
		sign := ""
		if amount < 0 {
			amount, sign = -amount, "-"
		}
		p.CellFormat(39, 7, sign+invoice.Currency+" "+money(amount), "", 1, "R", false, 0, "")
	}
	y := p.GetY() + 2
	p.SetFillColor(r, g, b)
	p.Rect(105, y, 87, 12, "F")
	p.SetTextColor(textOnAccent[0], textOnAccent[1], textOnAccent[2])
	p.SetFont("Arial", "B", 11)
	p.SetXY(109, y+2)
	p.CellFormat(44, 8, "BALANCE DUE", "", 0, "L", false, 0, "")
	p.CellFormat(35, 8, invoice.Currency+" "+money(invoice.BalanceMinor), "", 1, "R", false, 0, "")
	setBody()
	p.SetY(y + 20)
	if invoice.Notes != "" {
		p.SetFont("Arial", "B", 10)
		p.Cell(0, 6, "NOTES")
		p.Ln(7)
		p.SetFont("Arial", "", 10)
		p.MultiCell(174, 6, invoice.Notes, "", "L", false)
		p.Ln(5)
	}
	if paymentInstructions != "" {
		p.SetFont("Arial", "B", 10)
		p.Cell(0, 6, "HOW TO PAY")
		p.Ln(7)
		p.SetFont("Arial", "", 10)
		p.MultiCell(174, 6, paymentInstructions, "", "L", false)
	}
}

func invoiceTableHeader(p *fpdf.Fpdf, template string, r, g, b int, textOnAccent []int) {
	y := p.GetY()
	fill := template == "modern" || template == "executive" || template == "ledger"
	if fill {
		p.SetFillColor(r, g, b)
		p.Rect(18, y, 174, 10, "F")
		p.SetTextColor(textOnAccent[0], textOnAccent[1], textOnAccent[2])
	} else {
		p.SetTextColor(r, g, b)
		p.SetDrawColor(r, g, b)
		p.Line(18, y+10, 192, y+10)
	}
	p.SetFont("Arial", "B", 10)
	p.SetXY(20, y+2)
	p.Cell(104, 6, "DESCRIPTION")
	p.SetXY(127, y+2)
	p.CellFormat(23, 6, "QTY", "", 0, "R", false, 0, "")
	p.SetXY(151, y+2)
	p.CellFormat(39, 6, "AMOUNT", "", 0, "R", false, 0, "")
	p.SetTextColor(35, 48, 44)
	p.SetY(y + 10)
}
