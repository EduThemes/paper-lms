package service

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// B200: the export must be a structurally valid PDF (every xref offset
// lands on its object header) and long portfolios must paginate.
func TestRenderPDF_ValidXrefAndPagination(t *testing.T) {
	long := strings.Repeat("<p>Lorem ipsum dolor sit amet, consectetur adipiscing elit (with parens) \\ and a backslash.</p>", 120)
	svc := &PortfolioService{}
	lines := svc.portfolioPDFLines(
		&models.Portfolio{Title: "Jane’s Portfolio", Tagline: "Learner", Description: "<b>Hi</b> &amp; welcome"},
		[]models.PortfolioSection{
			{ID: 1, Title: "About", Content: long, IsVisible: true},
			{ID: 2, Title: "Hidden", Content: "secret", IsVisible: false},
		},
		[]models.PortfolioArtifact{{ID: 1, Title: "Essay", ContentURL: "https://example.com/essay"}},
	)
	pdf := renderPDF("Jane’s Portfolio", lines)

	require.True(t, bytes.HasPrefix(pdf, []byte("%PDF-1.4\n")))
	assert.NotContains(t, string(pdf), "secret", "hidden sections are not exported")
	assert.Contains(t, string(pdf), "Hi & welcome")

	count := regexp.MustCompile(`/Count (\d+)`).FindSubmatch(pdf)
	require.NotNil(t, count)
	pages, _ := strconv.Atoi(string(count[1]))
	assert.Greater(t, pages, 1, "long content must span several pages")

	startxref := regexp.MustCompile(`startxref\n(\d+)\n%%EOF\n$`).FindSubmatch(pdf)
	require.NotNil(t, startxref)
	xrefAt, _ := strconv.Atoi(string(startxref[1]))
	require.True(t, bytes.HasPrefix(pdf[xrefAt:], []byte("xref\n")))

	entries := regexp.MustCompile(`(\d{10}) 00000 n `).FindAllSubmatch(pdf[xrefAt:], -1)
	require.NotEmpty(t, entries)
	for i, e := range entries {
		off, _ := strconv.Atoi(string(e[1]))
		want := fmt.Sprintf("%d 0 obj\n", i+1)
		assert.True(t, bytes.HasPrefix(pdf[off:], []byte(want)), "xref entry %d must point at %q", i+1, want)
	}
}

func TestPDFEncode_EscapesAndMapsWinAnsi(t *testing.T) {
	assert.Equal(t, `a\(b\)\\c`, pdfEncode(`a(b)\c`))
	assert.Equal(t, "\x92", pdfEncode("’"))
	assert.Equal(t, "?", pdfEncode("漢"))
}
