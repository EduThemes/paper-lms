package service_test

// Page Designer data through Common Cartridge export/import and course
// copy (docs/page-designer/README.md). All data is made up.

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
)

const designBase = "https://paper.example.edu"

type designedCourse struct {
	course *models.Course
	page   *models.WikiPage
	plain  *models.WikiPage
	file   *models.Attachment
}

func (f *laneFixture) seedDesigned(t *testing.T, code string) *designedCourse {
	t.Helper()
	ctx := context.Background()
	d := &designedCourse{course: laneSeedCourse(t, f.g, 1, code)}
	d.file = f.uploadFile(t, d.course.ID, "banner.txt", "pretend banner")
	fileURL := fmt.Sprintf("/api/v1/files/%d/download", d.file.ID)

	pages := postgres.NewPageRepository(f.g)
	d.plain = &models.WikiPage{CourseID: d.course.ID, Title: "Syllabus notes", Body: "<p>Plain</p>", WorkflowState: "active"}
	require.NoError(t, pages.Create(ctx, d.plain))

	body := `<div id="pd-doc-1" class="paper-design" data-pd-v="1" style="background:var(--pd-c1, #12355b);color:var(--pd-c2, #ffffff)">` +
		`<img src="/design-assets/themes/tide-pool/banner.webp" srcset="/design-assets/themes/tide-pool/banner.webp 1x, /design-assets/themes/tide-pool/banner@2x.webp 2x" alt="">` +
		`<img src="/api/v1/design/icons/duotone/star.svg?fg=ffffff&amp;bg=12355b" alt="">` +
		`<p style="font-family:var(--reading-font-family, Georgia)">Welcome</p></div>`
	design := fmt.Sprintf(`{"version":1,"id":"doc-1","themeId":"tide-pool","sections":[{"type":"hero","image":%q,"html":"<a href=\"/courses/%d/pages/syllabus-notes\">Notes</a>"}]}`,
		fileURL, d.course.ID)
	d.page = &models.WikiPage{CourseID: d.course.ID, Title: "Home", Body: body, WorkflowState: "active",
		CSS: fmt.Sprintf(`.hero{background:url(%s)}`, fileURL), Design: models.NullJSON(design)}
	require.NoError(t, pages.Create(ctx, d.page))
	// Store the engine's body verbatim: the rich-content policy that keeps
	// these inline styles is another lane's work; this test is about export.
	require.NoError(t, f.g.Exec(`UPDATE wiki_pages SET body = ? WHERE id = ?`, body, d.page.ID).Error)
	d.page.Body = body

	d.course.Look = models.NullJSON(fmt.Sprintf(`{"version":1,"theme_id":"tide-pool","colors":{"primary":"#12355b"},"banner":{"src":%q,"alt":""}}`, fileURL))
	require.NoError(t, postgres.NewCourseRepository(f.g).Update(ctx, d.course))
	return d
}

func (f *laneFixture) parser(t *testing.T) *service.IMSCCParser {
	g := f.g
	fileService := service.NewFileServiceWithBackend(postgres.NewFolderRepository(g), postgres.NewAttachmentRepository(g), storage.NewLocalBackend(t.TempDir()))
	return service.NewIMSCCParser(
		postgres.NewCourseRepository(g), postgres.NewModuleRepository(g), postgres.NewModuleItemRepository(g), postgres.NewPageRepository(g), postgres.NewAssignmentRepository(g),
		postgres.NewQuizRepository(g), postgres.NewQuizQuestionRepository(g), fileService, postgres.NewFolderRepository(g), postgres.NewDiscussionTopicRepository(g),
		postgres.NewQuestionBankRepository(g), postgres.NewQuestionBankEntryRepository(g), postgres.NewAssignmentGroupRepository(g), postgres.NewAnnouncementRepository(g),
		postgres.NewRubricRepository(g), postgres.NewRubricAssociationRepository(g), postgres.NewLearningOutcomeGroupRepository(g), postgres.NewLearningOutcomeRepository(g), postgres.NewCalendarEventRepository(g),
	)
}

func TestIMSCC_PageDesignsExportForCanvasAndRoundTrip(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedDesigned(t, "DESIGN-1")

	zipPath, res, err := newLaneExporter(f).SetPublicBaseURL(designBase+"/").ExportCourse(ctx, src.course.ID, t.TempDir())
	require.NoError(t, err)
	require.Empty(t, res.Errors)
	raw, err := os.ReadFile(zipPath) // #nosec G304 -- test temp dir
	require.NoError(t, err)
	entries := zipEntries(t, raw)

	// The page HTML renders on its own in Canvas: light colours, absolute
	// designer URLs; other var()s and links are untouched.
	html := entries["wiki_content/home.html"]
	require.NotEmpty(t, html)
	assert.Contains(t, html, `style="background:#12355b;color:#ffffff"`)
	assert.NotContains(t, html, "var(--pd-")
	assert.Contains(t, html, "var(--reading-font-family, Georgia)")
	assert.Contains(t, html, `src="`+designBase+`/design-assets/themes/tide-pool/banner.webp"`)
	assert.Contains(t, html, designBase+`/design-assets/themes/tide-pool/banner@2x.webp 2x`)
	assert.Contains(t, html, `src="`+designBase+`/api/v1/design/icons/duotone/star.svg?fg=ffffff&amp;bg=12355b"`)

	// Paper-only metadata: the design (file and page links as cartridge
	// tokens), the stylesheet and the course look.
	meta := entries["course_settings/paper_designs.json"]
	require.NotEmpty(t, meta)
	var doc struct {
		Version int                       `json:"version"`
		Look    map[string]any            `json:"look"`
		Pages   map[string]map[string]any `json:"pages"`
	}
	require.NoError(t, json.Unmarshal([]byte(meta), &doc))
	assert.Equal(t, 1, doc.Version)
	entry := doc.Pages[fmt.Sprintf("res_page_%d", src.page.ID)]
	require.NotNil(t, entry)
	assert.NotContains(t, doc.Pages, fmt.Sprintf("res_page_%d", src.plain.ID), "ordinary pages carry nothing")
	designJSON, _ := json.Marshal(entry["design"])
	assert.Contains(t, string(designJSON), "$IMS-CC-FILEBASE$/")
	assert.Contains(t, string(designJSON), "$WIKI_REFERENCE$/wiki/syllabus-notes")
	assert.Contains(t, entry["css"], "url($IMS-CC-FILEBASE$/")
	assert.Contains(t, doc.Look["banner"].(map[string]any)["src"], "$IMS-CC-FILEBASE$/")

	// Paper → Paper: import into a fresh course.
	dst := laneSeedCourse(t, f.g, 1, "DESIGN-2")
	result, err := f.parser(t).SetPageDesigner(designBase, nil).ParsePackage(ctx, dst.ID, f.userID, zipPath)
	require.NoError(t, err)
	require.Empty(t, result.Errors)

	var imported []models.WikiPage
	require.NoError(t, f.g.Where("course_id = ?", dst.ID).Find(&imported).Error)
	var home *models.WikiPage
	for i := range imported {
		if imported[i].Title == "Home" {
			home = &imported[i]
		} else {
			assert.True(t, imported[i].Design.IsNull(), imported[i].Title)
		}
	}
	require.NotNil(t, home)
	require.False(t, home.Design.IsNull())

	var att models.Attachment
	require.NoError(t, f.g.Where("context_type = 'Course' AND context_id = ? AND display_name = ?", dst.ID, "banner.txt").First(&att).Error)
	newFileURL := fmt.Sprintf("/api/v1/files/%d/download", att.ID)

	var design map[string]any
	require.NoError(t, json.Unmarshal(home.Design, &design))
	assert.Equal(t, "doc-1", design["id"])
	section := design["sections"].([]any)[0].(map[string]any)
	assert.Equal(t, newFileURL, section["image"])
	assert.Contains(t, section["html"], fmt.Sprintf(`/courses/%d/pages/syllabus-notes`, dst.ID))
	assert.NotContains(t, string(home.Design), "$IMS-CC-FILEBASE$")

	// The body's designer URLs are local again on the instance that made them.
	assert.Contains(t, home.Body, `src="/design-assets/themes/tide-pool/banner.webp"`)
	assert.NotContains(t, home.Body, designBase)

	var course models.Course
	require.NoError(t, f.g.First(&course, dst.ID).Error)
	require.False(t, course.Look.IsNull())
	var look map[string]any
	require.NoError(t, json.Unmarshal(course.Look, &look))
	assert.Equal(t, "tide-pool", look["theme_id"])
	assert.Equal(t, newFileURL, look["banner"].(map[string]any)["src"])

}

func TestIMSCC_PaperDesignsAreRevalidatedOnImport(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedDesigned(t, "DESIGN-3")
	zipPath, _, err := newLaneExporter(f).ExportCourse(ctx, src.course.ID, t.TempDir())
	require.NoError(t, err)

	// Tamper with the cartridge: an invalid design and an external banner.
	tampered := rewriteZipEntry(t, zipPath, "course_settings/paper_designs.json", func(s string) string {
		var doc map[string]any
		require.NoError(t, json.Unmarshal([]byte(s), &doc))
		for _, page := range doc["pages"].(map[string]any) {
			page.(map[string]any)["design"].(map[string]any)["id"] = "<script>"
		}
		doc["look"].(map[string]any)["banner"].(map[string]any)["src"] = "https://evil.example/banner.webp"
		out, err := json.Marshal(doc)
		require.NoError(t, err)
		return string(out)
	})
	dst := laneSeedCourse(t, f.g, 1, "DESIGN-4")
	result, err := f.parser(t).ParsePackage(ctx, dst.ID, f.userID, tampered)
	require.NoError(t, err)

	var home models.WikiPage
	require.NoError(t, f.g.Where("course_id = ? AND title = ?", dst.ID, "Home").First(&home).Error)
	assert.True(t, home.Design.IsNull(), "an invalid design is dropped")
	var course models.Course
	require.NoError(t, f.g.First(&course, dst.ID).Error)
	assert.True(t, course.Look.IsNull(), "a look with an external banner is dropped")
	assert.NotEmpty(t, result.Warnings)
}

func TestCourseCopy_CarriesPageDesignsAndLook(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedDesigned(t, "DESIGN-5")
	dst := laneSeedCourse(t, f.g, 1, "DESIGN-6")

	_, err := f.copier().Copy(ctx, src.course.ID, dst.ID, f.userID, models.MigrationSettings{}, nil)
	require.NoError(t, err)

	var home models.WikiPage
	require.NoError(t, f.g.Where("course_id = ? AND title = ?", dst.ID, "Home").First(&home).Error)
	require.False(t, home.Design.IsNull())
	var att models.Attachment
	require.NoError(t, f.g.Where("context_type = 'Course' AND context_id = ? AND display_name = ?", dst.ID, "banner.txt").First(&att).Error)
	newFileURL := fmt.Sprintf("/api/v1/files/%d/download", att.ID)
	assert.Contains(t, string(home.Design), newFileURL, "file links inside the design follow the copied file")
	assert.Contains(t, home.CSS, newFileURL)

	var course models.Course
	require.NoError(t, f.g.First(&course, dst.ID).Error)
	assert.Contains(t, string(course.Look), newFileURL)
}

// rewriteZipEntry copies the cartridge at path with one entry rewritten.
func rewriteZipEntry(t *testing.T, path, name string, edit func(string) string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer func() { _ = zr.Close() }()
	out := filepath.Join(t.TempDir(), "tampered.imscc")
	fh, err := os.Create(out) // #nosec G304 -- test temp dir
	require.NoError(t, err)
	zw := zip.NewWriter(fh)
	found := false
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		_ = rc.Close()
		if f.Name == name {
			body = []byte(edit(string(body)))
			found = true
		}
		w, err := zw.Create(f.Name)
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}
	require.True(t, found, "entry %s not in cartridge", name)
	require.NoError(t, zw.Close())
	require.NoError(t, fh.Close())
	return out
}
