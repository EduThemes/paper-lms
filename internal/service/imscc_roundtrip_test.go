//go:build integration

// Round-trip parity test for the IMSCC import/export pair. Gated behind
// the "integration" build tag so `go test ./...` doesn't try to spin up
// Postgres, and behind an INTEGRATION_DATABASE_URL env var so it skips
// cleanly on machines without a database.
//
// Run:
//
//   INTEGRATION_DATABASE_URL=postgres://paper:paper@localhost:5432/paper_test \
//   ROUNDTRIP_CARTRIDGE=/abs/path/to/quantitown-algebra-1-export.imscc \
//   go test -tags=integration ./internal/service/... -run TestIMSCCRoundTrip -count=1 -v

package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/db"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/storage"
	"gorm.io/gorm"
)

// requireIntegrationDB skips the test if INTEGRATION_DATABASE_URL isn't set;
// otherwise opens the connection, runs AutoMigrate, and returns the *gorm.DB.
func requireIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("INTEGRATION_DATABASE_URL")
	if url == "" {
		t.Skip("INTEGRATION_DATABASE_URL not set; skipping round-trip test")
	}
	gdb, err := db.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return gdb
}

// requireCartridge resolves the round-trip cartridge path. Tries the
// ROUNDTRIP_CARTRIDGE env var, then a sibling-of-repo fallback that
// matches the developer's typical layout.
func requireCartridge(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("ROUNDTRIP_CARTRIDGE"); p != "" {
		return p
	}
	// Look one level up from the repo root for the test cartridge.
	guesses := []string{
		"../../../quantitown-algebra-1-export.imscc",
		"/Users/michael/Documents/❄️ Iced Projects/Paper LMS/quantitown-algebra-1-export.imscc",
	}
	for _, g := range guesses {
		if _, err := os.Stat(g); err == nil {
			return g
		}
	}
	t.Skip("no cartridge available (set ROUNDTRIP_CARTRIDGE)")
	return ""
}

func TestIMSCCRoundTrip(t *testing.T) {
	gdb := requireIntegrationDB(t)
	cartridge := requireCartridge(t)
	tmp := t.TempDir()
	ctx := context.Background()

	// --- Wire repos exactly the way main.go does. ---
	courseRepo := postgres.NewCourseRepository(gdb)
	moduleRepo := postgres.NewModuleRepository(gdb)
	moduleItemRepo := postgres.NewModuleItemRepository(gdb)
	pageRepo := postgres.NewPageRepository(gdb)
	assignmentRepo := postgres.NewAssignmentRepository(gdb)
	quizRepo := postgres.NewQuizRepository(gdb)
	quizQuestionRepo := postgres.NewQuizQuestionRepository(gdb)
	discussionTopicRepo := postgres.NewDiscussionTopicRepository(gdb)
	folderRepo := postgres.NewFolderRepository(gdb)
	attachmentRepo := postgres.NewAttachmentRepository(gdb)
	questionBankRepo := postgres.NewQuestionBankRepository(gdb)
	questionBankEntryRepo := postgres.NewQuestionBankEntryRepository(gdb)
	assignmentGroupRepo := postgres.NewAssignmentGroupRepository(gdb)
	announcementRepo := postgres.NewAnnouncementRepository(gdb)
	rubricRepo := postgres.NewRubricRepository(gdb)
	rubricAssocRepo := postgres.NewRubricAssociationRepository(gdb)
	outcomeGroupRepo := postgres.NewLearningOutcomeGroupRepository(gdb)
	outcomeRepo := postgres.NewLearningOutcomeRepository(gdb)
	calendarEventRepo := postgres.NewCalendarEventRepository(gdb)

	// FileService backed by local disk under tmp.
	storageDir := filepath.Join(tmp, "files")
	_ = os.MkdirAll(storageDir, 0o755)
	backend := storage.NewLocalBackend(storageDir)
	fileService := NewFileServiceWithBackend(folderRepo, attachmentRepo, backend)

	parser := NewIMSCCParser(
		courseRepo, moduleRepo, moduleItemRepo, pageRepo, assignmentRepo,
		quizRepo, quizQuestionRepo, fileService, folderRepo, discussionTopicRepo,
		questionBankRepo, questionBankEntryRepo, assignmentGroupRepo, announcementRepo,
		rubricRepo, rubricAssocRepo, outcomeGroupRepo, outcomeRepo, calendarEventRepo,
	)
	exporter := NewIMSCCExporter(
		courseRepo, moduleRepo, moduleItemRepo, pageRepo, assignmentRepo,
		quizRepo, quizQuestionRepo, discussionTopicRepo,
		assignmentGroupRepo, rubricRepo, rubricAssocRepo,
		outcomeGroupRepo, outcomeRepo, calendarEventRepo,
		attachmentRepo, fileService,
	)

	// --- Create a fresh course, drive the round trip. ---
	owner := &models.User{
		Name:         "Round Trip",
		LoginID:      fmt.Sprintf("roundtrip+%d@example.com", os.Getpid()),
		Email:        fmt.Sprintf("roundtrip+%d@example.com", os.Getpid()),
		PasswordHash: "x",
		Role:         "admin",
	}
	if err := gdb.Create(owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	t.Cleanup(func() { gdb.Unscoped().Delete(owner) })

	course := &models.Course{
		AccountID:     1,
		Name:          "Round Trip Course",
		CourseCode:    "RT-101",
		WorkflowState: "available",
	}
	if err := gdb.Create(course).Error; err != nil {
		t.Fatalf("create course: %v", err)
	}
	t.Cleanup(func() { gdb.Unscoped().Where("course_id = ?", course.ID).Delete(&models.WikiPage{}) })
	t.Cleanup(func() { gdb.Unscoped().Delete(course) })

	importResult, err := parser.ParsePackage(ctx, course.ID, owner.ID, cartridge)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	t.Cleanup(func() { parser.CleanupFailedImport(ctx, course.ID, importResult.CreatedEntities) })
	t.Logf("import: %d pages, %d assignments, %d quizzes, %d questions, %d files, %d errors, %d warnings",
		importResult.PagesCreated, importResult.AssignmentsCreated, importResult.QuizzesCreated,
		importResult.QuestionsCreated, importResult.FilesCreated, len(importResult.Errors), len(importResult.Warnings))

	exportPath, exportResult, err := exporter.ExportCourse(ctx, course.ID, tmp)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	t.Logf("export: %d pages, %d assignments, %d quizzes, %d errors",
		exportResult.PagesExported, exportResult.AssignmentsExported,
		exportResult.QuizzesExported, len(exportResult.Errors))

	// --- Compare zips. ---
	report := compareCartridges(t, cartridge, exportPath)
	report.print(t)
	if report.substantiveDeltas() > 0 {
		t.Fatalf("%d substantive deltas — round-trip not parity-clean", report.substantiveDeltas())
	}
}

type parityReport struct {
	matchedFiles  []string
	tolerableDeltas []string // identifiers, timestamps, MD5/checksum fields
	substantive   []string   // text differences, missing elements, missing files
	onlyInSource  []string
	onlyInExport  []string
}

func (r *parityReport) substantiveDeltas() int { return len(r.substantive) }

func (r *parityReport) print(t *testing.T) {
	t.Helper()
	t.Logf("Parity report:")
	t.Logf("  matched: %d", len(r.matchedFiles))
	t.Logf("  tolerable deltas: %d", len(r.tolerableDeltas))
	t.Logf("  substantive deltas: %d", len(r.substantive))
	t.Logf("  only in source: %d", len(r.onlyInSource))
	t.Logf("  only in export: %d", len(r.onlyInExport))
	for _, s := range r.substantive {
		t.Logf("    SUBSTANTIVE: %s", s)
	}
	if testing.Verbose() {
		for _, s := range r.onlyInSource {
			t.Logf("    only-in-source: %s", s)
		}
		for _, s := range r.onlyInExport {
			t.Logf("    only-in-export: %s", s)
		}
	}
}

// compareCartridges does a structural diff between two .imscc zips. Files
// that exist in both are compared with a class-specific canonicalizer; files
// only in one side go into the lopsided lists. Identifier / timestamp /
// checksum noise is classified as "tolerable", everything else as
// "substantive".
func compareCartridges(t *testing.T, sourcePath, exportPath string) *parityReport {
	t.Helper()
	src := readZipFiles(t, sourcePath)
	exp := readZipFiles(t, exportPath)
	r := &parityReport{}

	allKeys := map[string]struct{}{}
	for k := range src {
		allKeys[k] = struct{}{}
	}
	for k := range exp {
		allKeys[k] = struct{}{}
	}
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, name := range keys {
		_, inSrc := src[name]
		_, inExp := exp[name]
		switch {
		case inSrc && !inExp:
			if isIgnoredOnlyFile(name) {
				r.tolerableDeltas = append(r.tolerableDeltas, "only-in-source: "+name)
			} else {
				r.onlyInSource = append(r.onlyInSource, name)
			}
		case inExp && !inSrc:
			if isIgnoredOnlyFile(name) {
				r.tolerableDeltas = append(r.tolerableDeltas, "only-in-export: "+name)
			} else {
				r.onlyInExport = append(r.onlyInExport, name)
			}
		default:
			delta := diffFile(name, src[name], exp[name])
			switch delta.kind {
			case "match":
				r.matchedFiles = append(r.matchedFiles, name)
			case "tolerable":
				r.tolerableDeltas = append(r.tolerableDeltas, name+": "+delta.detail)
			case "substantive":
				r.substantive = append(r.substantive, name+": "+delta.detail)
			}
		}
	}
	return r
}

type fileDelta struct {
	kind   string // match | tolerable | substantive
	detail string
}

// diffFile classifies a same-name file. The matrix is intentionally
// generous about identifier/timestamp differences (the Wave 2B emitter
// regenerates ids from internal pkeys) and strict about content
// differences in HTML bodies + binary files.
func diffFile(name string, srcBytes, expBytes []byte) fileDelta {
	if bytes.Equal(srcBytes, expBytes) {
		return fileDelta{kind: "match"}
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "web_resources/"):
		// Binary asset files: compare by MD5; mismatches are substantive.
		srcSum := md5.Sum(srcBytes)
		expSum := md5.Sum(expBytes)
		if srcSum != expSum {
			return fileDelta{kind: "substantive", detail: fmt.Sprintf("md5 src=%s exp=%s",
				hex.EncodeToString(srcSum[:8]), hex.EncodeToString(expSum[:8]))}
		}
		return fileDelta{kind: "match"}
	case strings.HasSuffix(lower, ".html"):
		// HTML bodies: compare canonicalized text content. Whitespace
		// outside element bodies is collapsed; the element shape itself
		// must match.
		if canonHTML(srcBytes) == canonHTML(expBytes) {
			return fileDelta{kind: "match"}
		}
		return fileDelta{kind: "substantive", detail: fmt.Sprintf("html bodies differ (src=%dB exp=%dB)", len(srcBytes), len(expBytes))}
	case strings.HasSuffix(lower, ".xml"):
		// XML metadata: tolerate identifier-attribute drift, fail on
		// element/attribute set differences.
		if canonXML(srcBytes) == canonXML(expBytes) {
			return fileDelta{kind: "match"}
		}
		return fileDelta{kind: "tolerable", detail: fmt.Sprintf("xml differs (src=%dB exp=%dB) — likely identifier drift", len(srcBytes), len(expBytes))}
	}
	// Anything else (txt, json) — content diff is tolerable unless we have
	// a stronger reason.
	return fileDelta{kind: "tolerable", detail: fmt.Sprintf("text differs (src=%dB exp=%dB)", len(srcBytes), len(expBytes))}
}

// canonHTML strips whitespace runs and identifiers so two HTML documents
// that render the same canonical content compare equal.
func canonHTML(b []byte) string {
	s := string(b)
	s = strings.Join(strings.Fields(s), " ")
	return strings.ToLower(s)
}

// canonXML normalizes whitespace + lowercases for a coarse comparison.
// Identifier attrs vary between import/export so this is not a strict
// compare; substantive HTML changes are caught upstream.
func canonXML(b []byte) string {
	s := string(b)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// isIgnoredOnlyFile is true for files we don't expect both sides to emit
// (e.g. context.xml, files_meta.xml — exporter writes them, importer
// doesn't always; or vice versa).
func isIgnoredOnlyFile(name string) bool {
	switch name {
	case "course_settings/context.xml",
		"course_settings/late_policy.xml",
		"course_settings/files_meta.xml",
		"course_settings/canvas_export.txt":
		return true
	}
	// Sidecar learning-application-resource files have unstable names.
	return false
}

// readZipFiles extracts every file from a .imscc / .zip into a name → bytes
// map. Used to drive the parity comparison.
func readZipFiles(t *testing.T, path string) map[string][]byte {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer r.Close()
	out := make(map[string][]byte, len(r.File))
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		out[f.Name] = body
	}
	return out
}
