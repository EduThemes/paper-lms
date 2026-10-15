package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/designdoc"
	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Page Designer data in a Common Cartridge (docs/page-designer/README.md,
// "Export to Common Cartridge").
//
// A designed page's HTML is exported so it renders on its own: the
// engine's var(--pd-…, #light) colours are flattened to #light and the
// designer's /design-assets/ and /api/v1/design/icons/ URLs are made
// absolute (FRONTEND_URL). The page stylesheet, the design document and
// the course look have no Canvas equivalent, so they travel in Paper's own
// course_settings/paper_designs.json, which Canvas ignores and a
// Paper → Paper import reads back.

// paperDesignsPath is the cartridge entry holding Paper-only design data.
const paperDesignsPath = "course_settings/paper_designs.json"

// paperDesigns is the paper_designs.json document.
type paperDesigns struct {
	Version int `json:"version"`
	// Look is the course look (courses.look), when course settings were
	// exported.
	Look json.RawMessage `json:"look,omitempty"`
	// Pages maps a page's manifest resource identifier to its stylesheet
	// and design.
	Pages map[string]paperPageDesign `json:"pages,omitempty"`
}

type paperPageDesign struct {
	CSS    string          `json:"css,omitempty"`
	Design json.RawMessage `json:"design,omitempty"`
}

// SetPublicBaseURL sets the public frontend URL (FRONTEND_URL) that
// designer asset and icon URLs are made absolute against on export.
func (e *IMSCCExporter) SetPublicBaseURL(base string) *IMSCCExporter {
	e.publicBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
	return e
}

// exportPageBody is a page body as it goes into the cartridge (before the
// Canvas token emitter runs).
func (e *IMSCCExporter) exportPageBody(body string) string {
	return designdoc.ExportPageHTML(body, e.publicBaseURL)
}

var (
	// Exact file download URLs in a stylesheet or JSON document. No
	// trailing path is taken: in CSS a ")" or ";" may follow, and in JSON
	// an escaped quote ("\"") — the HTML emitter's regex would swallow it.
	reDesignFileURL = regexp.MustCompile(`/api/v1/files/(\d+)/download`)
	reDesignPageURL = regexp.MustCompile(`/courses/(\d+)/pages/([A-Za-z0-9_-]+)`)
	// The import side: tokens followed by a path in url.PathEscape's
	// alphabet (which never contains quotes, backslashes or parentheses).
	reDesignFileToken = regexp.MustCompile(`\$IMS-CC-FILEBASE\$/([A-Za-z0-9\-._~$&+,:;=@%]+)`)
	reDesignWikiToken = regexp.MustCompile(`\$WIKI_REFERENCE\$/wiki/([A-Za-z0-9_-]+)`)
)

// emitDesignTokens turns this course's file and page URLs in a stylesheet
// or JSON document into the cartridge's portable tokens.
func emitDesignTokens(s string, ctx *tokenEmitterCtx) string {
	if s == "" || ctx == nil {
		return s
	}
	s = reDesignFileURL.ReplaceAllStringFunc(s, func(m string) string {
		g := reDesignFileURL.FindStringSubmatch(m)
		id, err := strconv.ParseUint(g[1], 10, 64)
		if err != nil {
			return m
		}
		zipPath, ok := ctx.filePathByID[uint(id)]
		if !ok {
			return m
		}
		return "$IMS-CC-FILEBASE$/" + url.PathEscape(strings.TrimPrefix(zipPath, "web_resources/"))
	})
	return reDesignPageURL.ReplaceAllStringFunc(s, func(m string) string {
		g := reDesignPageURL.FindStringSubmatch(m)
		if g[1] != strconv.FormatUint(uint64(ctx.courseID), 10) || !ctx.pageSlugs[g[2]] {
			return m
		}
		return "$WIKI_REFERENCE$/wiki/" + g[2]
	})
}

// rewriteDesignTokens is the import-side inverse of emitDesignTokens.
// Unresolvable tokens are left in place, as rewriteTokens does.
func rewriteDesignTokens(s string, rc *tokenRewriteCtx) string {
	if s == "" || rc == nil {
		return s
	}
	s = reDesignFileToken.ReplaceAllStringFunc(s, func(m string) string {
		raw := reDesignFileToken.FindStringSubmatch(m)[1]
		candidates := []string{raw}
		if dec, err := url.PathUnescape(raw); err == nil && dec != raw {
			candidates = append(candidates, dec)
		}
		for _, c := range candidates {
			for _, key := range []string{c, strings.TrimPrefix(c, "/"), "web_resources/" + strings.TrimPrefix(c, "/")} {
				if u, ok := rc.fileURLByPath[key]; ok {
					return u
				}
			}
		}
		return m
	})
	return reDesignWikiToken.ReplaceAllStringFunc(s, func(m string) string {
		slug := reDesignWikiToken.FindStringSubmatch(m)[1]
		if _, ok := rc.pageBySlug[slug]; ok {
			return fmt.Sprintf("/courses/%d/pages/%s", rc.courseID, slug)
		}
		return m
	})
}

// marshalPaperDesigns encodes the document without HTML escaping (the
// stylesheets are full of ">" selectors).
func marshalPaperDesigns(doc *paperDesigns) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writePaperDesigns writes course_settings/paper_designs.json when there
// is anything to carry. pages holds the exported pages' designs keyed by
// resource identifier (already token-emitted).
func (e *IMSCCExporter) writePaperDesigns(w *zip.Writer, course *models.Course, withLook bool, pages map[string]paperPageDesign, ctx *tokenEmitterCtx) error {
	doc := &paperDesigns{Version: 1, Pages: pages}
	if withLook && course != nil && !course.Look.IsNull() {
		look := emitDesignTokens(string(course.Look), ctx)
		if json.Valid([]byte(look)) {
			doc.Look = json.RawMessage(look)
		}
	}
	if len(doc.Pages) == 0 && len(doc.Look) == 0 {
		return nil
	}
	body, err := marshalPaperDesigns(doc)
	if err != nil {
		return err
	}
	return writeRawFile(w, paperDesignsPath, body)
}

// exportPageDesign returns the page's paper_designs.json entry, or false
// for an ordinary page.
func exportPageDesign(page *models.WikiPage, ctx *tokenEmitterCtx) (paperPageDesign, bool) {
	if strings.TrimSpace(page.CSS) == "" && page.Design.IsNull() {
		return paperPageDesign{}, false
	}
	entry := paperPageDesign{CSS: emitDesignTokens(page.CSS, ctx)}
	if !page.Design.IsNull() {
		if d := emitDesignTokens(string(page.Design), ctx); json.Valid([]byte(d)) {
			entry.Design = json.RawMessage(d)
		}
	}
	return entry, true
}

// SetPageDesigner configures the import side: publicBaseURL (this
// instance's FRONTEND_URL, so a cartridge exported here gets its designer
// URLs made relative again) and customCSSAllowed, which reports whether
// pages.custom_css_enabled is on for the target course (nil: on).
func (p *IMSCCParser) SetPageDesigner(publicBaseURL string, customCSSAllowed func(ctx context.Context, courseID uint) bool) *IMSCCParser {
	p.publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	p.customCSSAllowed = customCSSAllowed
	return p
}

// applyPaperDesigns reads course_settings/paper_designs.json (written by a
// Paper export) and restores page stylesheets and designs and the course
// look. Runs after pass-2, so every page exists and file tokens resolve.
// Everything is re-validated: a cartridge is untrusted input.
func (p *IMSCCParser) applyPaperDesigns(ctx context.Context, courseID uint, zipFiles map[string]*zip.File, result *ImportResult) {
	data, err := readZipFile(zipFiles, paperDesignsPath)
	if err != nil || len(data) == 0 {
		return
	}
	var doc paperDesigns
	if err := json.Unmarshal(data, &doc); err != nil || doc.Version != 1 {
		result.Warnings = append(result.Warnings, "paper_designs.json was not readable; page designs were not imported")
		return
	}
	rc := &tokenRewriteCtx{
		courseID:      courseID,
		fileURLByPath: result.fileURLByPath,
		pageBySlug:    result.pageBySlug,
		entityByMigID: result.entityByMigID,
	}
	cssAllowed := true
	if p.customCSSAllowed != nil {
		cssAllowed = p.customCSSAllowed(ctx, courseID)
	}

	for resID, entry := range doc.Pages {
		ref, ok := result.entityByMigID[resID]
		if !ok || ref.Type != "WikiPage" {
			continue
		}
		// accountID=0: auth-internal — a page this import just created.
		page, err := p.pageRepo.FindByID(ctx, ref.ID, 0)
		if err != nil || page == nil || page.CourseID != courseID {
			continue
		}
		page.Design = nil
		if len(entry.Design) > 0 {
			d, err := designdoc.ValidateDesign([]byte(rewriteDesignTokens(string(entry.Design), rc)))
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("page %q: design not imported: %v", page.Title, err))
			} else {
				page.Design = d
			}
		}
		page.CSS = resanitizePageCSS(rewriteDesignTokens(entry.CSS, rc))
		if !cssAllowed && page.Design.IsNull() {
			page.CSS = ""
		}
		page.Body = designdoc.RelativizeDesignURLs(page.Body, p.publicBaseURL)
		if err := p.pageRepo.Update(ctx, page); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %q: design not saved: %v", page.Title, err))
		}
	}

	if len(doc.Look) > 0 && !models.NullJSON(doc.Look).IsNull() {
		imported := map[uint]bool{}
		for _, id := range result.attachmentByPath {
			imported[id] = true
		}
		look, err := designdoc.ValidateLook([]byte(rewriteDesignTokens(string(doc.Look), rc)), func(id uint) bool { return imported[id] })
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("course look not imported: %v", err))
			return
		}
		course, err := p.courseRepo.FindByID(ctx, courseID, 0)
		if err != nil || course == nil {
			return
		}
		course.Look = look
		if err := p.courseRepo.Update(ctx, course); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("course look not saved: %v", err))
		}
	}
}
