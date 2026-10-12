package service_test

// G068: importing a Canvas cartridge keeps module unlock dates,
// prerequisites, "complete one" requirement counts, item completion
// requirements and the web_resources folder structure.

import (
	"archive/zip"
	"context"
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

const g068Manifest = `<?xml version="1.0" encoding="UTF-8"?>
<manifest identifier="cc_g068" xmlns="http://www.imsglobal.org/xsd/imsccv1p1/imscp_v1p1">
  <organizations>
    <organization identifier="org_1" structure="rooted-hierarchy">
      <item identifier="LearningModules">
        <item identifier="mod_1">
          <title>Week 1</title>
          <item identifier="item_1" identifierref="res_page1"><title>Welcome</title></item>
          <item identifier="item_2" identifierref="res_page2"><title>Reading</title></item>
        </item>
        <item identifier="mod_2">
          <title>Week 2</title>
          <item identifier="item_3" identifierref="res_page3"><title>Lab</title></item>
          <item identifier="item_4" identifierref="res_page4"><title>Extra</title></item>
        </item>
      </item>
    </organization>
  </organizations>
  <resources>
    <resource identifier="res_page1" type="webcontent" href="wiki_content/welcome.html"><file href="wiki_content/welcome.html"/></resource>
    <resource identifier="res_page2" type="webcontent" href="wiki_content/reading.html"><file href="wiki_content/reading.html"/></resource>
    <resource identifier="res_page3" type="webcontent" href="wiki_content/lab.html"><file href="wiki_content/lab.html"/></resource>
    <resource identifier="res_page4" type="webcontent" href="wiki_content/extra.html"><file href="wiki_content/extra.html"/></resource>
  </resources>
</manifest>`

const g068ModuleMeta = `<?xml version="1.0" encoding="UTF-8"?>
<modules xmlns="http://canvas.instructure.com/xsd/cccv1p0">
  <module identifier="mod_1">
    <title>Week 1</title>
    <workflow_state>active</workflow_state>
    <require_sequential_progress>true</require_sequential_progress>
    <items>
      <item identifier="item_1"><content_type>WikiPage</content_type><identifierref>res_page1</identifierref></item>
      <item identifier="item_2"><content_type>WikiPage</content_type><identifierref>res_page2</identifierref></item>
    </items>
    <completionRequirements>
      <completionRequirement type="must_view"><identifierref>item_1</identifierref></completionRequirement>
      <completionRequirement type="must_mark_done"><identifierref>item_2</identifierref></completionRequirement>
    </completionRequirements>
  </module>
  <module identifier="mod_2">
    <title>Week 2</title>
    <workflow_state>active</workflow_state>
    <unlock_at>2031-01-15T08:00:00Z</unlock_at>
    <requirement_count>1</requirement_count>
    <prerequisites>
      <prerequisite type="context_module"><title>Week 1</title><identifierref>mod_1</identifierref></prerequisite>
      <prerequisite type="context_module"><title>Gone</title><identifierref>mod_missing</identifierref></prerequisite>
    </prerequisites>
    <items>
      <item identifier="item_3"><content_type>WikiPage</content_type><identifierref>res_page3</identifierref></item>
      <item identifier="item_4"><content_type>WikiPage</content_type><identifierref>res_page4</identifierref></item>
    </items>
    <completionRequirements>
      <completionRequirement type="must_view"><identifierref>item_3</identifierref></completionRequirement>
      <completionRequirement type="must_submit"><identifierref>item_4</identifierref></completionRequirement>
    </completionRequirements>
  </module>
</modules>`

func writeG068Cartridge(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "g068.imscc")
	f, err := os.Create(p)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	files := map[string]string{
		"imsmanifest.xml":                    g068Manifest,
		"course_settings/module_meta.xml":    g068ModuleMeta,
		"wiki_content/welcome.html":          "<html><head><title>Welcome</title></head><body><p>Hi</p></body></html>",
		"wiki_content/reading.html":          "<html><head><title>Reading</title></head><body><p>Read</p></body></html>",
		"wiki_content/lab.html":              "<html><head><title>Lab</title></head><body><p>Lab</p></body></html>",
		"wiki_content/extra.html":            "<html><head><title>Extra</title></head><body><p>Extra</p></body></html>",
		"web_resources/handouts/unit1/a.pdf": "%PDF-1.4 a",
		"web_resources/handouts/unit1/b.pdf": "%PDF-1.4 b",
		"web_resources/top.pdf":              "%PDF-1.4 top",
	}
	for name, body := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	return p
}

func TestIMSCCImport_ModuleMetaSettings(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	course := laneSeedCourse(t, g, 1, "G068")
	owner := &models.User{AccountID: 1, Name: "Importer", LoginID: "importer@example.test", Email: "importer@example.test", PasswordHash: "x", Role: "user"}
	require.NoError(t, g.Create(owner).Error)

	folderRepo := postgres.NewFolderRepository(g)
	attachmentRepo := postgres.NewAttachmentRepository(g)
	fileService := service.NewFileServiceWithBackend(folderRepo, attachmentRepo, storage.NewLocalBackend(t.TempDir()))
	moduleRepo := postgres.NewModuleRepository(g)
	moduleItemRepo := postgres.NewModuleItemRepository(g)
	parser := service.NewIMSCCParser(
		postgres.NewCourseRepository(g), moduleRepo, moduleItemRepo, postgres.NewPageRepository(g), postgres.NewAssignmentRepository(g),
		postgres.NewQuizRepository(g), postgres.NewQuizQuestionRepository(g), fileService, folderRepo, postgres.NewDiscussionTopicRepository(g),
		postgres.NewQuestionBankRepository(g), postgres.NewQuestionBankEntryRepository(g), postgres.NewAssignmentGroupRepository(g), postgres.NewAnnouncementRepository(g),
		postgres.NewRubricRepository(g), postgres.NewRubricAssociationRepository(g), postgres.NewLearningOutcomeGroupRepository(g), postgres.NewLearningOutcomeRepository(g), postgres.NewCalendarEventRepository(g),
	)
	prereqRepo := postgres.NewModulePrerequisiteRepository(g)
	parser.SetPrerequisiteRepo(prereqRepo)

	result, err := parser.ParsePackage(ctx, course.ID, owner.ID, writeG068Cartridge(t))
	require.NoError(t, err)
	require.Empty(t, result.Errors)
	assert.Equal(t, 2, result.ModulesCreated)

	var mods []models.ContextModule
	require.NoError(t, g.Where("course_id = ?", course.ID).Order("position").Find(&mods).Error)
	require.Len(t, mods, 2)
	week1, week2 := mods[0], mods[1]
	assert.True(t, week1.RequireSequentialProgress)
	assert.Nil(t, week1.UnlockAt)
	require.NotNil(t, week2.UnlockAt)
	assert.Equal(t, "2031-01-15T08:00:00Z", week2.UnlockAt.UTC().Format("2006-01-02T15:04:05Z"))
	assert.Equal(t, 1, week2.RequirementCount)

	prereqs, err := prereqRepo.GetPrerequisites(ctx, week2.ID)
	require.NoError(t, err)
	assert.Equal(t, []uint{week1.ID}, prereqs, "the unresolvable prerequisite is skipped")

	reqs := map[string]models.ContentTag{}
	var tags []models.ContentTag
	require.NoError(t, g.Where("context_module_id IN ?", []uint{week1.ID, week2.ID}).Find(&tags).Error)
	for _, tag := range tags {
		reqs[tag.Title] = tag
	}
	assert.Equal(t, models.ModuleCompletionMustView, reqs["Welcome"].CompletionType)
	assert.Equal(t, models.ModuleCompletionMustMarkDone, reqs["Reading"].CompletionType)
	assert.Equal(t, models.ModuleCompletionMustView, reqs["Lab"].CompletionType)
	assert.Equal(t, "", reqs["Extra"].CompletionType, "must_submit is not valid on a page")
	assert.NotEmpty(t, result.Warnings)

	// web_resources/handouts/unit1/*.pdf keep their folders; top.pdf stays at the root.
	var folders []models.Folder
	require.NoError(t, g.Where("context_type = ? AND context_id = ?", "Course", course.ID).Find(&folders).Error)
	byName := map[string]models.Folder{}
	for _, f := range folders {
		byName[f.FullName] = f
	}
	root, ok := byName["course files"]
	require.True(t, ok)
	handouts, ok := byName["course files/handouts"]
	require.True(t, ok)
	unit1, ok := byName["course files/handouts/unit1"]
	require.True(t, ok)
	assert.Equal(t, root.ID, *handouts.ParentFolderID)
	assert.Equal(t, handouts.ID, *unit1.ParentFolderID)

	var atts []models.Attachment
	require.NoError(t, g.Where("context_type = ? AND context_id = ?", "Course", course.ID).Find(&atts).Error)
	folderOf := map[string]uint{}
	for _, a := range atts {
		folderOf[a.DisplayName] = *a.FolderID
	}
	assert.Equal(t, unit1.ID, folderOf["a.pdf"])
	assert.Equal(t, unit1.ID, folderOf["b.pdf"])
	assert.Equal(t, root.ID, folderOf["top.pdf"])
}
