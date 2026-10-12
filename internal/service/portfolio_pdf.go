package service

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Minimal, dependency-free PDF writer for portfolio export (B200: the
// "Export as PDF" endpoint used to return HTML that the browser saved as a
// .pdf file no viewer could open). It lays the portfolio out as plain text
// with the PDF base-14 Helvetica fonts, wrapping lines and paginating on US
// Letter pages. Rich section HTML is reduced to text.

const (
	pdfPageWidth  = 612.0
	pdfPageHeight = 792.0
	pdfMargin     = 72.0
)

type pdfFont int

const (
	pdfRegular pdfFont = iota + 1
	pdfBold
	pdfItalic
)

type pdfLine struct {
	font pdfFont
	size float64
	text string
	gap  float64 // extra space before the line
}

var (
	pdfBlockTagRe = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li|/h[1-6]|/tr|/blockquote)\b[^>]*>`)
	pdfLiTagRe    = regexp.MustCompile(`(?i)<\s*li\b[^>]*>`)
	pdfAnyTagRe   = regexp.MustCompile(`<[^>]*>`)
	pdfSpaceRe    = regexp.MustCompile(`[ \t\r\f\v]+`)
)

// htmlToPlainText reduces rich HTML to paragraphs of plain text.
func htmlToPlainText(s string) []string {
	s = pdfBlockTagRe.ReplaceAllString(s, "\n")
	s = pdfLiTagRe.ReplaceAllString(s, "\n- ")
	s = pdfAnyTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(pdfSpaceRe.ReplaceAllString(line, " "))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// wrapText splits text into lines that fit the printable width at size,
// using an average Helvetica glyph width (conservative for mixed text).
func wrapText(text string, size float64) []string {
	maxChars := int((pdfPageWidth - 2*pdfMargin) / (size * 0.5))
	if maxChars < 10 {
		maxChars = 10
	}
	words := strings.Fields(text)
	var lines []string
	cur := ""
	for _, w := range words {
		for len([]rune(w)) > maxChars {
			r := []rune(w)
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, string(r[:maxChars]))
			w = string(r[maxChars:])
		}
		switch {
		case cur == "":
			cur = w
		case len([]rune(cur))+1+len([]rune(w)) <= maxChars:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// pdfEncode converts text to WinAnsiEncoding bytes and escapes it for a PDF
// literal string. Characters outside WinAnsi become '?'.
func pdfEncode(s string) string {
	var b strings.Builder
	for _, r := range s {
		var c byte
		switch {
		case r == '\\' || r == '(' || r == ')':
			b.WriteByte('\\')
			c = byte(r)
		case r >= 32 && r < 127:
			c = byte(r)
		case r >= 160 && r <= 255:
			c = byte(r)
		case r == '‘':
			c = 0x91
		case r == '’':
			c = 0x92
		case r == '“':
			c = 0x93
		case r == '”':
			c = 0x94
		case r == '•':
			c = 0x95
		case r == '–':
			c = 0x96
		case r == '—':
			c = 0x97
		case r == '…':
			c = 0x85
		case r == '€':
			c = 0x80
		default:
			c = '?'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func (s *PortfolioService) portfolioPDFLines(portfolio *models.Portfolio, sections []models.PortfolioSection, artifacts []models.PortfolioArtifact) []pdfLine {
	var lines []pdfLine
	add := func(font pdfFont, size float64, text string, gap float64) {
		for i, l := range wrapText(text, size) {
			g := 0.0
			if i == 0 {
				g = gap
			}
			lines = append(lines, pdfLine{font: font, size: size, text: l, gap: g})
		}
	}
	addArtifact := func(a models.PortfolioArtifact) {
		add(pdfBold, 11, a.Title, 6)
		for _, p := range htmlToPlainText(a.Description) {
			add(pdfRegular, 10, p, 0)
		}
		if a.ContentURL != "" {
			add(pdfItalic, 9, a.ContentURL, 0)
		}
	}

	add(pdfBold, 20, portfolio.Title, 0)
	if portfolio.Tagline != "" {
		add(pdfItalic, 12, portfolio.Tagline, 6)
	}
	for _, p := range htmlToPlainText(portfolio.Description) {
		add(pdfRegular, 11, p, 6)
	}
	for _, sec := range sections {
		if !sec.IsVisible {
			continue
		}
		add(pdfBold, 14, sec.Title, 18)
		for _, p := range htmlToPlainText(sec.Content) {
			add(pdfRegular, 11, p, 6)
		}
		for _, a := range artifacts {
			if a.SectionID != nil && *a.SectionID == sec.ID {
				addArtifact(a)
			}
		}
	}
	var loose []models.PortfolioArtifact
	for _, a := range artifacts {
		if a.SectionID == nil {
			loose = append(loose, a)
		}
	}
	if len(loose) > 0 {
		add(pdfBold, 14, "Portfolio Artifacts", 18)
		for _, a := range loose {
			addArtifact(a)
		}
	}
	return lines
}

// renderPDF lays out lines on pages and serializes a PDF 1.4 document.
func renderPDF(title string, lines []pdfLine) []byte {
	// Paginate.
	var pages [][]string // content stream operators per page
	var cur []string
	y := pdfPageHeight - pdfMargin
	for _, l := range lines {
		lead := l.size * 1.35
		if y-l.gap-lead < pdfMargin && len(cur) > 0 {
			pages = append(pages, cur)
			cur = nil
			y = pdfPageHeight - pdfMargin
		} else {
			y -= l.gap
		}
		y -= lead
		cur = append(cur, fmt.Sprintf("BT /F%d %.1f Tf %.2f %.2f Td (%s) Tj ET", l.font, l.size, pdfMargin, y, pdfEncode(l.text)))
	}
	if len(cur) > 0 || len(pages) == 0 {
		pages = append(pages, cur)
	}

	var buf bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}

	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	// 1: catalog, 2: pages, 3-5: fonts, 6: info, then page/content pairs.
	const firstPageObj = 7
	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", firstPageObj+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Oblique /Encoding /WinAnsiEncoding >>")
	obj(fmt.Sprintf("<< /Title (%s) /Producer (Paper LMS) >>", pdfEncode(title)))
	for i, ops := range pages {
		content := strings.Join(ops, "\n")
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>",
			pdfPageWidth, pdfPageHeight, firstPageObj+2*i+1))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R /Info 6 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes()
}
