package service_test

// Content-migration lane: course copy (G067), content export (G066/G142),
// blueprint deletions + locks (G069) and Commons fidelity (G081).

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
)

var lanePage = repository.PaginationParams{Page: 1, PerPage: 500}

func laneTime(t time.Time) *time.Time { return &t }

// ------------------------------------------------------------ date shift

func TestDateShifter_ProportionalKeepsWeekdayAndTime(t *testing.T) {
	opts := &models.DateShiftOptions{
		ShiftDates:   true,
		OldStartDate: laneTime(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)),
		OldEndDate:   laneTime(time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)),
		NewStartDate: laneTime(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)),
		NewEndDate:   laneTime(time.Date(2026, 12, 14, 0, 0, 0, 0, time.UTC)),
	}
	d := service.NewDateShifter(opts, nil, nil)
	require.NotNil(t, d)

	// Wednesday 2025-09-17 23:59 → Wednesday 2026-09-16 23:59.
	got := d.Shift(time.Date(2025, 9, 17, 23, 59, 0, 0, time.UTC))
	require.NotNil(t, got)
	assert.True(t, got.Equal(time.Date(2026, 9, 16, 23, 59, 0, 0, time.UTC)), got.String())
	assert.Equal(t, time.Wednesday, got.Weekday())

	// Wednesday → Thursday substitution picks the earlier Thursday that
	// is still inside the new window.
	opts.DaySubstitutions = map[string]string{"3": "4"}
	d = service.NewDateShifter(opts, nil, nil)
	got = d.Shift(time.Date(2025, 9, 17, 23, 59, 0, 0, time.UTC))
	assert.True(t, got.Equal(time.Date(2026, 9, 10, 23, 59, 0, 0, time.UTC)), got.String())
}

func TestDateShifter_OffsetTimeZoneAndRemoval(t *testing.T) {
	// Only start dates: shift by whole days, computed in the course zone.
	opts := &models.DateShiftOptions{
		ShiftDates:   true,
		OldStartDate: laneTime(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)),
		NewStartDate: laneTime(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)),
		TimeZone:     "America/New_York",
	}
	d := service.NewDateShifter(opts, nil, nil)
	require.NotNil(t, d)
	// 03:59 UTC Thursday is 23:59 Wednesday in New York.
	got := d.Shift(time.Date(2025, 9, 18, 3, 59, 0, 0, time.UTC))
	require.NotNil(t, got)
	assert.True(t, got.Equal(time.Date(2026, 9, 17, 3, 59, 0, 0, time.UTC)), got.UTC().String())

	// Missing dates fall back to the source and target courses.
	src := &models.Course{StartAt: laneTime(time.Date(2025, 9, 1, 12, 0, 0, 0, time.UTC)), EndAt: laneTime(time.Date(2025, 12, 15, 12, 0, 0, 0, time.UTC))}
	dst := &models.Course{StartAt: laneTime(time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC))}
	d = service.NewDateShifter(&models.DateShiftOptions{ShiftDates: true}, src, dst)
	require.NotNil(t, d)
	got = d.Shift(time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC))
	assert.True(t, got.Equal(time.Date(2026, 12, 14, 10, 0, 0, 0, time.UTC)), got.String())

	// Nothing to shift from.
	assert.Nil(t, service.NewDateShifter(&models.DateShiftOptions{ShiftDates: true}, nil, nil))
	assert.Nil(t, service.NewDateShifter(&models.DateShiftOptions{}, src, dst))

	rm := service.NewDateShifter(&models.DateShiftOptions{RemoveDates: true}, nil, nil)
	assert.Nil(t, rm.Shift(time.Now()))
	start := time.Now()
	assert.Equal(t, start, rm.ShiftRequired(start), "a required date is never cleared")
}

func TestDateShiftOptions_AcceptsPlainDates(t *testing.T) {
	var s models.MigrationSettings
	require.NoError(t, json.Unmarshal([]byte(`{"date_shift_options":{"shift_dates":true,"old_start_date":"2025-09-01","new_start_date":"2026-08-31T00:00:00Z","day_substitutions":{"1":"2"}}}`), &s))
	require.NotNil(t, s.DateShiftOptions)
	assert.True(t, s.DateShiftOptions.ShiftDates)
	assert.True(t, s.DateShiftOptions.OldStartDate.Equal(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)))
	assert.True(t, s.DateShiftOptions.NewStartDate.Equal(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, "2", s.DateShiftOptions.DaySubstitutions["1"])

	err := json.Unmarshal([]byte(`{"date_shift_options":{"old_start_date":"next tuesday"}}`), &s)
	assert.Error(t, err)
}

// ------------------------------------------------------------ fixtures

type laneFixture struct {
	g       *gorm.DB
	files   *service.FileService
	backend storage.Backend
	userID  uint
}

func newLaneFixture(t *testing.T) *laneFixture {
	g, cleanup := freshDB(t)
	t.Cleanup(cleanup)
	laneSeedCourse(t, g, 1, "SEED")
	u := &models.User{AccountID: 1, Name: "Tess Teacher", LoginID: "tess.lane@example.test", Email: "tess.lane@example.test"}
	require.NoError(t, g.Create(u).Error)
	backend := storage.NewLocalBackend(t.TempDir())
	files := service.NewFileServiceWithBackend(postgres.NewFolderRepository(g), postgres.NewAttachmentRepository(g), backend)
	return &laneFixture{g: g, files: files, backend: backend, userID: u.ID}
}

func (f *laneFixture) copier() *service.CourseCopyService {
	g := f.g
	return service.NewCourseCopyService(service.CourseCopyDeps{
		Courses:          postgres.NewCourseRepository(g),
		AssignmentGroups: postgres.NewAssignmentGroupRepository(g),
		Assignments:      postgres.NewAssignmentRepository(g),
		Quizzes:          postgres.NewQuizRepository(g),
		Questions:        postgres.NewQuizQuestionRepository(g),
		QuestionGroups:   postgres.NewQuizQuestionGroupRepository(g),
		Pages:            postgres.NewPageRepository(g),
		Discussions:      postgres.NewDiscussionTopicRepository(g),
		Modules:          postgres.NewModuleRepository(g),
		ModuleItems:      postgres.NewModuleItemRepository(g),
		Events:           postgres.NewCalendarEventRepository(g),
		Rubrics:          postgres.NewRubricRepository(g),
		RubricAssocs:     postgres.NewRubricAssociationRepository(g),
		Folders:          postgres.NewFolderRepository(g),
		Attachments:      postgres.NewAttachmentRepository(g),
		Files:            f.files,
	})
}

func (f *laneFixture) uploadFile(t *testing.T, courseID uint, name, body string) *models.Attachment {
	t.Helper()
	root, err := f.files.GetOrCreateRootFolder(context.Background(), "Course", courseID)
	require.NoError(t, err)
	att := &models.Attachment{ContextType: "Course", ContextID: courseID, FolderID: &root.ID, UserID: f.userID,
		DisplayName: name, Filename: name, ContentType: "text/plain", Size: int64(len(body))}
	require.NoError(t, f.files.UploadFile(context.Background(), att, strings.NewReader(body)))
	return att
}

// sourceCourse seeds a course with one of everything; returns the ids.
type seeded struct {
	course     *models.Course
	group      *models.AssignmentGroup
	essay      *models.Assignment
	quiz       *models.Quiz
	page       *models.WikiPage
	discussion *models.DiscussionTopic
	module     *models.ContextModule
	event      *models.CalendarEvent
	file       *models.Attachment
	rubric     *models.Rubric
}

func (f *laneFixture) seedSource(t *testing.T, code string) *seeded {
	t.Helper()
	ctx := context.Background()
	g := f.g
	s := &seeded{course: laneSeedCourse(t, g, 1, code)}
	s.file = f.uploadFile(t, s.course.ID, "syllabus-notes.txt", "Bring a lab notebook.")

	s.group = &models.AssignmentGroup{CourseID: s.course.ID, Name: "Labs", GroupWeight: 40, Rules: "{}", WorkflowState: "available"}
	require.NoError(t, postgres.NewAssignmentGroupRepository(g).Create(ctx, s.group))

	pts := 20.0
	due := time.Date(2025, 9, 17, 23, 59, 0, 0, time.UTC)
	s.essay = &models.Assignment{CourseID: s.course.ID, Name: "Lab report", Description: "<p>Use the template.</p>",
		PointsPossible: &pts, DueAt: &due, AssignmentGroupID: &s.group.ID, WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, postgres.NewAssignmentRepository(g).Create(ctx, s.essay))

	s.rubric = &models.Rubric{ContextType: "Course", ContextID: s.course.ID, Title: "Lab rubric", Data: `[{"id":"c1","points":20}]`, PointsPossible: 20, WorkflowState: "active"}
	require.NoError(t, postgres.NewRubricRepository(g).Create(ctx, s.rubric))
	require.NoError(t, postgres.NewRubricAssociationRepository(g).Create(ctx, &models.RubricAssociation{
		RubricID: s.rubric.ID, AssociationID: s.essay.ID, AssociationType: "Assignment", ContextType: "Course", ContextID: s.course.ID, Purpose: "grading", UseForGrading: true}))

	s.quiz = &models.Quiz{CourseID: s.course.ID, Title: "Cells quiz", QuizType: "assignment", AllowedAttempts: 2, WorkflowState: "available", Published: true, DueAt: &due}
	require.NoError(t, postgres.NewQuizRepository(g).Create(ctx, s.quiz))
	s.quiz.ShowCorrectAnswers = false
	require.NoError(t, postgres.NewQuizRepository(g).Update(ctx, s.quiz))
	grp := &models.QuizQuestionGroup{QuizID: s.quiz.ID, Name: "Pick one", PickCount: 1}
	require.NoError(t, postgres.NewQuizQuestionGroupRepository(g).Create(ctx, grp))
	qq := postgres.NewQuizQuestionRepository(g)
	require.NoError(t, qq.Create(ctx, &models.QuizQuestion{QuizID: s.quiz.ID, Position: 1, QuestionType: "essay_question", QuestionText: "What is a cell?", Answers: "[]", WorkflowState: "active"}))
	require.NoError(t, qq.Create(ctx, &models.QuizQuestion{QuizID: s.quiz.ID, QuizQuestionGroupID: &grp.ID, Position: 2, QuestionType: "essay_question", QuestionText: "Name an organelle.", Answers: "[]", WorkflowState: "active"}))

	body := fmt.Sprintf(`<p>Start with <a href="/courses/%d/assignments/%d">the lab</a>, then <a href="/courses/%d/quizzes/%d">the quiz</a>. Notes: <a href="/api/v1/files/%d/download">file</a>. <a href="/courses/%d/modules">Modules</a>.</p>`,
		s.course.ID, s.essay.ID, s.course.ID, s.quiz.ID, s.file.ID, s.course.ID)
	s.page = &models.WikiPage{CourseID: s.course.ID, Title: "Start here", Body: body, WorkflowState: "active", FrontPage: true}
	require.NoError(t, postgres.NewPageRepository(g).Create(ctx, s.page))

	s.discussion = &models.DiscussionTopic{CourseID: s.course.ID, UserID: f.userID, Title: "Lab questions", Message: "<p>Ask here</p>", AssignmentID: &s.essay.ID, WorkflowState: models.DiscussionTopicActive}
	require.NoError(t, postgres.NewDiscussionTopicRepository(g).Create(ctx, s.discussion))

	s.event = &models.CalendarEvent{ContextType: "Course", ContextID: s.course.ID, Title: "Lab day", StartAt: time.Date(2025, 9, 15, 14, 0, 0, 0, time.UTC), CreatedByUserID: f.userID, WorkflowState: "active"}
	require.NoError(t, postgres.NewCalendarEventRepository(g).Create(ctx, s.event))

	s.module = &models.ContextModule{CourseID: s.course.ID, Name: "Unit 1", Position: 1, WorkflowState: "active"}
	require.NoError(t, postgres.NewModuleRepository(g).Create(ctx, s.module))
	items := postgres.NewModuleItemRepository(g)
	require.NoError(t, items.Create(ctx, &models.ContentTag{ContextModuleID: s.module.ID, ContentType: "Assignment", ContentID: &s.essay.ID, Title: "Lab report", Position: 1, WorkflowState: "active"}))
	require.NoError(t, items.Create(ctx, &models.ContentTag{ContextModuleID: s.module.ID, ContentType: "WikiPage", ContentID: &s.page.ID, URL: s.page.URL, Title: "Start here", Position: 2, WorkflowState: "active"}))
	require.NoError(t, items.Create(ctx, &models.ContentTag{ContextModuleID: s.module.ID, ContentType: "Attachment", ContentID: &s.file.ID, Title: "Notes", Position: 3, WorkflowState: "active"}))
	require.NoError(t, items.Create(ctx, &models.ContentTag{ContextModuleID: s.module.ID, ContentType: "ContextModuleSubHeader", Title: "Readings", Position: 4, WorkflowState: "active"}))
	return s
}

// ------------------------------------------------------------ course copy

func TestCourseCopy_CopiesEverythingWithLinksAndSettings(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedSource(t, "BIO-101-F25")
	dst := laneSeedCourse(t, f.g, 1, "BIO-101-F26")
	existing := &models.WikiPage{CourseID: dst.ID, Title: "Home", Body: "<p>Hi</p>", WorkflowState: "active", FrontPage: true}
	require.NoError(t, postgres.NewPageRepository(f.g).Create(ctx, existing))

	res, err := f.copier().Copy(ctx, src.course.ID, dst.ID, f.userID, models.MigrationSettings{}, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Counts[service.CopyTypeAssignments])
	assert.Equal(t, 1, res.Counts[service.CopyTypeQuizzes])
	assert.Equal(t, 2, res.Counts["quiz_questions"])
	assert.Equal(t, 1, res.Counts[service.CopyTypePages])
	assert.Equal(t, 1, res.Counts[service.CopyTypeDiscussions])
	assert.Equal(t, 1, res.Counts[service.CopyTypeModules])
	assert.Equal(t, 4, res.Counts["module_items"])
	assert.Equal(t, 1, res.Counts[service.CopyTypeFiles])
	assert.Equal(t, 1, res.Counts[service.CopyTypeEvents])
	assert.Equal(t, 1, res.Counts[service.CopyTypeRubrics])
	assert.Empty(t, res.Warnings)

	// Assignment kept its group (the copy) and its rubric.
	as, err := postgres.NewAssignmentRepository(f.g).ListByCourseID(ctx, dst.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, as.Items, 1)
	copyA := as.Items[0]
	require.NotNil(t, copyA.AssignmentGroupID)
	grp, err := postgres.NewAssignmentGroupRepository(f.g).FindByID(ctx, *copyA.AssignmentGroupID, 1)
	require.NoError(t, err)
	assert.Equal(t, dst.ID, grp.CourseID)
	assert.Equal(t, "Labs", grp.Name)
	ra, err := postgres.NewRubricAssociationRepository(f.g).FindByAssociation(ctx, copyA.ID, "Assignment")
	require.NoError(t, err)
	assert.NotEqual(t, src.rubric.ID, ra.RubricID)

	// Quiz keeps show_correct_answers=false, its group and questions.
	qs, err := postgres.NewQuizRepository(f.g).ListByCourseID(ctx, dst.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, qs.Items, 1)
	assert.False(t, qs.Items[0].ShowCorrectAnswers)
	assert.Equal(t, 2, qs.Items[0].AllowedAttempts)
	groups, err := postgres.NewQuizQuestionGroupRepository(f.g).ListByQuizID(ctx, qs.Items[0].ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	questions, err := postgres.NewQuizQuestionRepository(f.g).ListByQuizID(ctx, qs.Items[0].ID, lanePage)
	require.NoError(t, err)
	require.Len(t, questions.Items, 2)
	grouped := 0
	for _, q := range questions.Items {
		if q.QuizQuestionGroupID != nil {
			assert.Equal(t, groups[0].ID, *q.QuizQuestionGroupID)
			grouped++
		}
	}
	assert.Equal(t, 1, grouped)

	// The page's links point at the copies; the target keeps its own
	// front page.
	pages, err := postgres.NewPageRepository(f.g).ListByCourseID(ctx, dst.ID, lanePage)
	require.NoError(t, err)
	var copied *models.WikiPage
	for i := range pages.Items {
		if pages.Items[i].Title == "Start here" {
			copied = &pages.Items[i]
		}
	}
	require.NotNil(t, copied)
	assert.False(t, copied.FrontPage)
	files, err := postgres.NewAttachmentRepository(f.g).ListByContext(ctx, "Course", dst.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, files.Items, 1)
	assert.Contains(t, copied.Body, fmt.Sprintf("/courses/%d/assignments/%d", dst.ID, copyA.ID))
	assert.Contains(t, copied.Body, fmt.Sprintf("/courses/%d/quizzes/%d", dst.ID, qs.Items[0].ID))
	assert.Contains(t, copied.Body, fmt.Sprintf("/api/v1/files/%d/download", files.Items[0].ID))
	assert.Contains(t, copied.Body, fmt.Sprintf("/courses/%d/modules", dst.ID))
	assert.NotContains(t, copied.Body, fmt.Sprintf("/courses/%d/", src.course.ID))

	// The file bytes were copied in storage.
	rc, err := f.backend.Get(ctx, files.Items[0].StoragePath)
	require.NoError(t, err)
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	assert.Equal(t, "Bring a lab notebook.", string(b))

	// Graded discussion points at the copied assignment; owned by the copier.
	ds, err := postgres.NewDiscussionTopicRepository(f.g).ListByCourseID(ctx, dst.ID, 0, lanePage)
	require.NoError(t, err)
	require.Len(t, ds.Items, 1)
	require.NotNil(t, ds.Items[0].AssignmentID)
	assert.Equal(t, copyA.ID, *ds.Items[0].AssignmentID)

	// Module items point at the copies.
	mods, err := postgres.NewModuleRepository(f.g).ListByCourseID(ctx, dst.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, mods.Items, 1)
	require.Len(t, mods.Items[0].Items, 4)
	for _, it := range mods.Items[0].Items {
		switch it.ContentType {
		case "Assignment":
			assert.Equal(t, copyA.ID, *it.ContentID)
		case "WikiPage":
			assert.Equal(t, copied.ID, *it.ContentID)
			assert.Equal(t, copied.URL, it.URL)
		case "Attachment":
			assert.Equal(t, files.Items[0].ID, *it.ContentID)
		}
	}
}

func TestCourseCopy_SelectiveWithDependenciesAndDateShift(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedSource(t, "CHEM-1")
	dst := laneSeedCourse(t, f.g, 1, "CHEM-2")

	settings := models.MigrationSettings{
		Select: map[string][]uint{"modules": {src.module.ID}, "calendar_events": {src.event.ID}},
		DateShiftOptions: &models.DateShiftOptions{
			ShiftDates:   true,
			OldStartDate: laneTime(time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)),
			NewStartDate: laneTime(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)),
		},
	}
	var seen []int
	res, err := f.copier().Copy(ctx, src.course.ID, dst.ID, f.userID, settings, func(p int) { seen = append(seen, p) })
	require.NoError(t, err)
	assert.Equal(t, 100, seen[len(seen)-1])

	// The module brought its assignment (and the assignment its group and
	// rubric), its page and its file — not the quiz or discussion.
	assert.Equal(t, 1, res.Counts[service.CopyTypeModules])
	assert.Equal(t, 1, res.Counts[service.CopyTypeAssignments])
	assert.Equal(t, 1, res.Counts[service.CopyTypeAssignmentGroups])
	assert.Equal(t, 1, res.Counts[service.CopyTypePages])
	assert.Equal(t, 1, res.Counts[service.CopyTypeFiles])
	assert.Equal(t, 0, res.Counts[service.CopyTypeQuizzes])
	assert.Equal(t, 0, res.Counts[service.CopyTypeDiscussions])
	assert.Equal(t, 1, res.Counts[service.CopyTypeEvents])
	assert.Equal(t, 2, res.Shifted, "assignment due date and event start")

	as, err := postgres.NewAssignmentRepository(f.g).ListByCourseID(ctx, dst.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, as.Items, 1)
	require.NotNil(t, as.Items[0].DueAt)
	assert.True(t, as.Items[0].DueAt.Equal(time.Date(2026, 9, 16, 23, 59, 0, 0, time.UTC)), as.Items[0].DueAt.String())

	// The page's quiz link stays on the source course (the quiz was not
	// copied) while the assignment link moves.
	pages, err := postgres.NewPageRepository(f.g).ListByCourseID(ctx, dst.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, pages.Items, 1)
	assert.Contains(t, pages.Items[0].Body, fmt.Sprintf("/courses/%d/quizzes/%d", src.course.ID, src.quiz.ID))
	assert.Contains(t, pages.Items[0].Body, fmt.Sprintf("/courses/%d/assignments/%d", dst.ID, as.Items[0].ID))

	// Exclusion wins over inclusion.
	dst2 := laneSeedCourse(t, f.g, 1, "CHEM-3")
	res, err = f.copier().Copy(ctx, src.course.ID, dst2.ID, f.userID, models.MigrationSettings{CopyEverything: true, ExcludeTypes: []string{"quizzes", "attachments"}}, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Counts[service.CopyTypeQuizzes])
	assert.Equal(t, 0, res.Counts[service.CopyTypeFiles])
	assert.Equal(t, 1, res.Counts[service.CopyTypeAssignments])
	assert.NotEmpty(t, res.Warnings, "module items linking excluded content are reported")
}

func TestCourseCopy_RefusesOtherTenantAndSelf(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := laneSeedCourse(t, f.g, 1, "MATH-1")
	foreign := laneSeedCourse(t, f.g, 2, "MATH-X")
	_, err := f.copier().Copy(ctx, src.ID, foreign.ID, f.userID, models.MigrationSettings{}, nil)
	assert.ErrorIs(t, err, service.ErrCourseCopyAcrossTenants)
	_, err = f.copier().Copy(ctx, src.ID, src.ID, f.userID, models.MigrationSettings{}, nil)
	assert.Error(t, err)
}

func TestContentMigrationRunner_RunsQueuedCourseCopyOnce(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedSource(t, "HIST-1")
	dst := laneSeedCourse(t, f.g, 1, "HIST-2")
	g := f.g
	migRepo := postgres.NewContentMigrationRepository(g)
	runner := service.NewContentMigrationRunner(migRepo, postgres.NewContentMigrationQueue(g), postgres.NewCourseRepository(g), f.copier(), nil,
		service.ContentDateRepos{Assignments: postgres.NewAssignmentRepository(g)})
	var queued []func()
	runner.SetDispatcher(func(fn func()) { queued = append(queued, fn) })

	srcID := src.course.ID
	m := &models.ContentMigration{CourseID: dst.ID, UserID: f.userID, MigrationType: "course_copy", SourceCourseID: &srcID,
		WorkflowState:     models.ContentMigrationQueued,
		MigrationSettings: datatypes.NewJSONType(models.MigrationSettings{IncludeOnlyTypes: []string{"assignments", "pages"}})}
	require.True(t, runner.Runnable(m))
	require.NoError(t, migRepo.Create(ctx, m))
	runner.Enqueue(m.ID)
	require.Len(t, queued, 1)
	queued[0]()

	got, err := migRepo.FindByID(ctx, m.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ContentMigrationCompleted, got.WorkflowState)
	assert.Equal(t, 100, got.Progress)
	assert.NotNil(t, got.FinishedAt)
	assert.Equal(t, 1, got.Results.Data().Counts["assignments"])
	assert.Equal(t, 1, got.Results.Data().Counts["pages"])

	// A second run (or the sweep) does not copy again.
	require.NoError(t, runner.Run(ctx, m.ID))
	require.NoError(t, runner.ProcessPending(ctx))
	as, err := postgres.NewAssignmentRepository(g).ListByCourseID(ctx, dst.ID, lanePage)
	require.NoError(t, err)
	assert.Len(t, as.Items, 1)

	// A copy from a missing course fails cleanly.
	missing := uint(987654)
	bad := &models.ContentMigration{CourseID: dst.ID, UserID: f.userID, MigrationType: "course_copy", SourceCourseID: &missing, WorkflowState: models.ContentMigrationQueued}
	require.NoError(t, migRepo.Create(ctx, bad))
	assert.Error(t, runner.Run(ctx, bad.ID))
	got, err = migRepo.FindByID(ctx, bad.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ContentMigrationFailed, got.WorkflowState)
	assert.NotEmpty(t, got.ErrorMessage)
}

// ------------------------------------------------------------ export

func newLaneExporter(f *laneFixture) *service.IMSCCExporter {
	g := f.g
	return service.NewIMSCCExporter(
		postgres.NewCourseRepository(g), postgres.NewModuleRepository(g), postgres.NewModuleItemRepository(g),
		postgres.NewPageRepository(g), postgres.NewAssignmentRepository(g), postgres.NewQuizRepository(g),
		postgres.NewQuizQuestionRepository(g), postgres.NewDiscussionTopicRepository(g), postgres.NewAssignmentGroupRepository(g),
		postgres.NewRubricRepository(g), postgres.NewRubricAssociationRepository(g),
		postgres.NewLearningOutcomeGroupRepository(g), postgres.NewLearningOutcomeRepository(g),
		postgres.NewCalendarEventRepository(g), postgres.NewAttachmentRepository(g), f.files,
	)
}

func zipEntries(t *testing.T, b []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	require.NoError(t, err)
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		body, _ := io.ReadAll(rc)
		_ = rc.Close()
		out[f.Name] = string(body)
	}
	return out
}

func TestContentExport_BuildsStoresAndSelects(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedSource(t, "ART-1")
	svc := service.NewContentExportService(postgres.NewContentExportRepository(f.g), newLaneExporter(f), f.backend, t.TempDir())
	svc.SetDispatcher(func(fn func()) { fn() })

	export, err := svc.Create(ctx, src.course.ID, f.userID, "", nil)
	require.NoError(t, err)
	got, err := svc.Get(ctx, export.ID, 1)
	require.NoError(t, err)
	require.Equal(t, models.ContentExportExported, got.WorkflowState, got.ErrorMessage)
	assert.Equal(t, 100, got.Progress)
	assert.True(t, strings.HasSuffix(got.FileName, ".imscc"))
	assert.Positive(t, got.FileSize)

	rc, err := svc.Open(ctx, got)
	require.NoError(t, err)
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	entries := zipEntries(t, b)
	require.Contains(t, entries, "imsmanifest.xml")
	assert.Contains(t, entries["imsmanifest.xml"], fmt.Sprintf("res_assignment_%d", src.essay.ID))
	assert.Contains(t, entries["imsmanifest.xml"], fmt.Sprintf("res_quiz_%d", src.quiz.ID))

	// Another tenant cannot read it.
	_, err = svc.Get(ctx, export.ID, 2)
	assert.Error(t, err)

	// Selective: only the quiz.
	sel, err := svc.Create(ctx, src.course.ID, f.userID, "common_cartridge", &models.MigrationSettings{Select: map[string][]uint{"quizzes": {src.quiz.ID}}})
	require.NoError(t, err)
	got, err = svc.Get(ctx, sel.ID, 1)
	require.NoError(t, err)
	require.Equal(t, models.ContentExportExported, got.WorkflowState, got.ErrorMessage)
	rc, err = svc.Open(ctx, got)
	require.NoError(t, err)
	b, _ = io.ReadAll(rc)
	_ = rc.Close()
	entries = zipEntries(t, b)
	assert.Contains(t, entries["imsmanifest.xml"], fmt.Sprintf("res_quiz_%d", src.quiz.ID))
	assert.NotContains(t, entries["imsmanifest.xml"], fmt.Sprintf("res_assignment_%d", src.essay.ID))
	assert.NotContains(t, entries["imsmanifest.xml"], fmt.Sprintf("res_page_%d", src.page.ID))

	_, err = svc.Create(ctx, src.course.ID, f.userID, "qti", nil)
	assert.ErrorIs(t, err, service.ErrInvalidExportType)

	// A not-yet-run export cannot be opened.
	_, err = svc.Open(ctx, &models.ContentExport{WorkflowState: models.ContentExportCreated})
	assert.ErrorIs(t, err, service.ErrContentExportNotReady)
}

// ------------------------------------------------------------ blueprint

func newLockingBlueprintService(f *laneFixture) *service.BlueprintService {
	svc := newLaneBlueprintService(f.g)
	svc.SetLockRepo(postgres.NewBlueprintContentLockRepository(f.g))
	svc.SetSyncDeps(postgres.NewAssignmentGroupRepository(f.g), postgres.NewAttachmentRepository(f.g), f.files)
	svc.SetRubricDeps(postgres.NewRubricRepository(f.g), postgres.NewRubricAssociationRepository(f.g))
	return svc
}

func TestBlueprintSync_PropagatesDeletionsGroupsFilesAndLinks(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedSource(t, "BP-DEL")
	child := laneSeedCourse(t, f.g, 1, "SEC-DEL")
	svc := newLockingBlueprintService(f)
	tmpl, err := svc.GetOrCreateTemplate(ctx, src.course.ID)
	require.NoError(t, err)
	require.NoError(t, svc.ModifyAssociations(ctx, tmpl, service.BlueprintCaller{UserID: 1, AccountID: 1, IsAdmin: true}, []uint{child.ID}, nil))
	_, err = svc.TriggerSync(ctx, tmpl.ID, f.userID, "initial")
	require.NoError(t, err)

	assignRepo := postgres.NewAssignmentRepository(f.g)
	pageRepo := postgres.NewPageRepository(f.g)
	childAs, err := assignRepo.ListByCourseID(ctx, child.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, childAs.Items, 1)
	require.NotNil(t, childAs.Items[0].AssignmentGroupID, "the copy sits in the child copy of its group")
	grp, err := postgres.NewAssignmentGroupRepository(f.g).FindByID(ctx, *childAs.Items[0].AssignmentGroupID, 1)
	require.NoError(t, err)
	assert.Equal(t, child.ID, grp.CourseID)
	childFiles, err := postgres.NewAttachmentRepository(f.g).ListByContext(ctx, "Course", child.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, childFiles.Items, 1)
	childPages, err := pageRepo.ListByCourseID(ctx, child.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, childPages.Items, 1)
	assert.Contains(t, childPages.Items[0].Body, fmt.Sprintf("/courses/%d/assignments/%d", child.ID, childAs.Items[0].ID))
	assert.Contains(t, childPages.Items[0].Body, fmt.Sprintf("/api/v1/files/%d/download", childFiles.Items[0].ID))

	// The rubric came along and grades the assignment copy.
	ra, err := postgres.NewRubricAssociationRepository(f.g).FindByAssociation(ctx, childAs.Items[0].ID, "Assignment")
	require.NoError(t, err)
	childRubric, err := postgres.NewRubricRepository(f.g).FindByID(ctx, ra.RubricID, 1)
	require.NoError(t, err)
	assert.Equal(t, child.ID, childRubric.ContextID)
	assert.Equal(t, "Lab rubric", childRubric.Title)

	// Course settings only travel when the sync asks for them.
	src.course.DefaultView = "syllabus"
	require.NoError(t, postgres.NewCourseRepository(f.g).Update(ctx, src.course))
	_, err = svc.TriggerSync(ctx, tmpl.ID, f.userID, "no settings")
	require.NoError(t, err)
	gotChild, err := postgres.NewCourseRepository(f.g).FindByID(ctx, child.ID, 1)
	require.NoError(t, err)
	assert.NotEqual(t, "syllabus", gotChild.DefaultView)
	_, err = svc.TriggerSyncWithOptions(ctx, tmpl.ID, f.userID, "with settings", service.BlueprintSyncOptions{CopySettings: true})
	require.NoError(t, err)
	gotChild, err = postgres.NewCourseRepository(f.g).FindByID(ctx, child.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "syllabus", gotChild.DefaultView)

	// Nothing pending right after a sync (the link rewrite is not a
	// "local edit").
	changes, err := svc.GetUnsyncedChanges(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Empty(t, changes)

	// The teacher edits the child's discussion; the blueprint deletes the
	// assignment and the discussion.
	discRepo := postgres.NewDiscussionTopicRepository(f.g)
	childDs, err := discRepo.ListByCourseID(ctx, child.ID, 0, lanePage)
	require.NoError(t, err)
	require.Len(t, childDs.Items, 1)
	time.Sleep(1100 * time.Millisecond)
	local := childDs.Items[0]
	local.Message = "<p>Section 2: office hours moved</p>"
	require.NoError(t, discRepo.Update(ctx, &local))
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, assignRepo.Delete(ctx, src.essay.ID))
	require.NoError(t, discRepo.Delete(ctx, src.discussion.ID))
	require.NoError(t, postgres.NewAssignmentGroupRepository(f.g).Delete(ctx, src.group.ID))

	changes, err = svc.GetUnsyncedChanges(ctx, tmpl.ID)
	require.NoError(t, err)
	deleted := map[string]uint{}
	for _, ch := range changes {
		if ch.ChangeType == "deleted" {
			deleted[ch.AssetType] = ch.AssetID
		}
	}
	assert.Equal(t, map[string]uint{"assignment": src.essay.ID, "discussion_topic": src.discussion.ID}, deleted)

	mig, err := svc.TriggerSync(ctx, tmpl.ID, f.userID, "deletions")
	require.NoError(t, err)
	assert.Contains(t, mig.ExportSettings, `"deleted":`)

	childAs, err = assignRepo.ListByCourseID(ctx, child.ID, lanePage)
	require.NoError(t, err)
	assert.Empty(t, childAs.Items, "an unchanged copy follows the blueprint deletion")
	childGroups, err := postgres.NewAssignmentGroupRepository(f.g).ListByCourseID(ctx, child.ID, lanePage)
	require.NoError(t, err)
	for _, cg := range childGroups.Items {
		assert.NotEqual(t, "Labs", cg.Name, "the emptied group copy follows its deletion")
	}
	childDs, err = discRepo.ListByCourseID(ctx, child.ID, 0, lanePage)
	require.NoError(t, err)
	require.Len(t, childDs.Items, 1, "a locally edited, unlocked copy is kept")
	assert.Equal(t, "<p>Section 2: office hours moved</p>", childDs.Items[0].Message)

	// The module item for the deleted assignment went with it.
	mods, err := postgres.NewModuleRepository(f.g).ListByCourseID(ctx, child.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, mods.Items, 1)
	for _, it := range mods.Items[0].Items {
		assert.NotEqual(t, "Assignment", it.ContentType)
	}
}

func TestBlueprintLocks_ForceSyncAndGuardChildEdits(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	bp := laneSeedCourse(t, f.g, 1, "BP-LOCK")
	child := laneSeedCourse(t, f.g, 1, "SEC-LOCK")
	assignRepo := postgres.NewAssignmentRepository(f.g)
	pageRepo := postgres.NewPageRepository(f.g)
	pts := 10.0
	due := time.Date(2026, 10, 1, 23, 59, 0, 0, time.UTC)
	essay := &models.Assignment{CourseID: bp.ID, Name: "Essay", Description: "<p>v1</p>", PointsPossible: &pts, DueAt: &due, WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, assignRepo.Create(ctx, essay))
	page := &models.WikiPage{CourseID: bp.ID, Title: "Policies", Body: "<p>v1</p>", WorkflowState: "active"}
	require.NoError(t, pageRepo.Create(ctx, page))

	svc := newLockingBlueprintService(f)
	tmpl, err := svc.GetOrCreateTemplate(ctx, bp.ID)
	require.NoError(t, err)
	require.NoError(t, svc.ModifyAssociations(ctx, tmpl, service.BlueprintCaller{UserID: 1, AccountID: 1, IsAdmin: true}, []uint{child.ID}, nil))

	// Lock the assignment's points + content; the page stays unlocked.
	require.NoError(t, svc.SetItemRestrictions(ctx, tmpl, 1, "Assignment", essay.ID, &service.BlueprintRestrictions{Content: true, Points: true}))
	// Content of another course cannot be locked through this template.
	other := &models.Assignment{CourseID: child.ID, Name: "Local", WorkflowState: models.AssignmentPublished}
	require.NoError(t, assignRepo.Create(ctx, other))
	assert.ErrorIs(t, svc.SetItemRestrictions(ctx, tmpl, 1, "Assignment", other.ID, &service.BlueprintRestrictions{Content: true}), service.ErrBlueprintLockInvalidContent)

	items, err := svc.ListLockableContent(ctx, tmpl)
	require.NoError(t, err)
	lockedSeen := false
	for _, it := range items {
		if it.ContentType == "Assignment" && it.ContentID == essay.ID {
			lockedSeen = it.Locked && it.Explicit && it.Restrictions.Points
		}
		if it.ContentType == "WikiPage" {
			assert.False(t, it.Locked)
		}
	}
	assert.True(t, lockedSeen)

	_, err = svc.TriggerSync(ctx, tmpl.ID, f.userID, "initial")
	require.NoError(t, err)
	childAs, err := assignRepo.ListByCourseID(ctx, child.ID, lanePage)
	require.NoError(t, err)
	var copyA models.Assignment
	for _, a := range childAs.Items {
		if a.Name == "Essay" {
			copyA = a
		}
	}
	require.NotZero(t, copyA.ID)

	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	// A locked attribute change is refused, naming the categories.
	err = svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "Assignment", ChildID: copyA.ID,
		Changes: map[string]json.RawMessage{"points_possible": raw(25), "name": raw("Essay")}})
	var locked *service.BlueprintLockedError
	require.ErrorAs(t, err, &locked)
	assert.Equal(t, []string{"points"}, locked.Attributes)
	// Re-sending unchanged values (a full form save) and editing an
	// unlocked attribute (due date) are fine.
	assert.NoError(t, svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "Assignment", ChildID: copyA.ID,
		Changes: map[string]json.RawMessage{"points_possible": raw(10), "name": raw("Essay"), "due_at": raw("2026-10-08T23:59:00Z")}}))
	// Deleting a locked copy is refused.
	assert.ErrorAs(t, svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "Assignment", ChildID: copyA.ID, Delete: true}), &locked)
	// The unlocked page and a non-copy are free.
	childPages, err := pageRepo.ListByCourseID(ctx, child.ID, lanePage)
	require.NoError(t, err)
	require.Len(t, childPages.Items, 1)
	assert.NoError(t, svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "WikiPage", ChildID: childPages.Items[0].ID, Delete: true}))
	assert.NoError(t, svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "Assignment", ChildID: other.ID, Delete: true}))

	lockedList, err := svc.ListChildLockedContent(ctx, child.ID)
	require.NoError(t, err)
	require.Len(t, lockedList, 1)
	assert.Equal(t, copyA.ID, lockedList[0].ChildID)

	// A local edit to the locked copy (made by other means) is overwritten
	// by the next sync; the unlocked page keeps its local edit.
	time.Sleep(1100 * time.Millisecond)
	copyA.Description = "<p>local</p>"
	require.NoError(t, assignRepo.Update(ctx, &copyA))
	localPage := childPages.Items[0]
	localPage.Body = "<p>local page</p>"
	require.NoError(t, pageRepo.Update(ctx, &localPage))
	time.Sleep(1100 * time.Millisecond)
	essay.Description = "<p>v2</p>"
	require.NoError(t, assignRepo.Update(ctx, essay))
	page.Body = "<p>v2</p>"
	require.NoError(t, pageRepo.Update(ctx, page))
	_, err = svc.TriggerSync(ctx, tmpl.ID, f.userID, "v2")
	require.NoError(t, err)
	gotA, err := assignRepo.FindByID(ctx, copyA.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, "<p>v2</p>", gotA.Description)
	gotP, err := pageRepo.FindByID(ctx, localPage.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, "<p>local page</p>", gotP.Body)

	// Inherit: dropping the item's own lock falls back to the (empty)
	// template defaults.
	require.NoError(t, svc.SetItemRestrictions(ctx, tmpl, 1, "Assignment", essay.ID, nil))
	assert.NoError(t, svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "Assignment", ChildID: copyA.ID, Delete: true}))

	// Default restrictions lock everything; detaching the course unlocks.
	tmpl.DefaultRestrictions = `{"content":true}`
	require.NoError(t, svc.UpdateTemplate(ctx, tmpl))
	assert.Error(t, svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "WikiPage", ChildID: localPage.ID, Delete: true}))
	require.NoError(t, svc.ModifyAssociations(ctx, tmpl, service.BlueprintCaller{UserID: 1, AccountID: 1, IsAdmin: true}, nil, []uint{child.ID}))
	assert.NoError(t, svc.CheckChildEdit(ctx, child.ID, service.ChildEdit{ContentType: "WikiPage", ChildID: localPage.ID, Delete: true}))
}

// ------------------------------------------------------------ commons

func TestCommonsImport_CarriesFilesGroupsLinksAndSettings(t *testing.T) {
	f := newLaneFixture(t)
	ctx := context.Background()
	src := f.seedSource(t, "COM-SRC")
	dst := laneSeedCourse(t, f.g, 1, "COM-DST")
	g := f.g
	svc := service.NewCommonsService(
		postgres.NewSharedContentRepository(g), postgres.NewCourseRepository(g), postgres.NewAssignmentRepository(g),
		postgres.NewPageRepository(g), postgres.NewQuizRepository(g), postgres.NewModuleRepository(g), postgres.NewDiscussionTopicRepository(g),
	)
	svc.SetContentDeps(postgres.NewQuizQuestionRepository(g), postgres.NewModuleItemRepository(g))
	svc.SetFileDeps(postgres.NewQuizQuestionGroupRepository(g), postgres.NewAttachmentRepository(g), f.files)

	// A quiz keeps its question group and show_correct_answers=false.
	shared, err := svc.Publish(ctx, f.userID, src.course.ID, 1, service.CommonsPublishOptions{ResourceType: "quiz", ResourceID: src.quiz.ID, Title: "Cells quiz"})
	require.NoError(t, err)
	res, err := svc.Import(ctx, f.userID, dst.ID, shared.ID, 1)
	require.NoError(t, err)
	require.Len(t, res.CreatedIDs["quizzes"], 1)
	q, err := postgres.NewQuizRepository(g).FindByID(ctx, res.CreatedIDs["quizzes"][0], 1)
	require.NoError(t, err)
	assert.False(t, q.ShowCorrectAnswers)
	groups, err := postgres.NewQuizQuestionGroupRepository(g).ListByQuizID(ctx, q.ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	qs, err := postgres.NewQuizQuestionRepository(g).ListByQuizID(ctx, q.ID, lanePage)
	require.NoError(t, err)
	grouped := 0
	for _, qq := range qs.Items {
		if qq.QuizQuestionGroupID != nil && *qq.QuizQuestionGroupID == groups[0].ID {
			grouped++
		}
	}
	assert.Equal(t, 1, grouped)

	// A module brings its file (module item + body link), and links
	// inside the page point at the imported copies.
	shared, err = svc.Publish(ctx, f.userID, src.course.ID, 1, service.CommonsPublishOptions{ResourceType: "module", ResourceID: src.module.ID, Title: "Unit 1"})
	require.NoError(t, err)
	res, err = svc.Import(ctx, f.userID, dst.ID, shared.ID, 1)
	require.NoError(t, err)
	require.Len(t, res.CreatedIDs["files"], 1)
	require.Len(t, res.CreatedIDs["pages"], 1)
	require.Len(t, res.CreatedIDs["assignments"], 1)
	p, err := postgres.NewPageRepository(g).FindByID(ctx, res.CreatedIDs["pages"][0], 1)
	require.NoError(t, err)
	assert.Contains(t, p.Body, fmt.Sprintf("/api/v1/files/%d/download", res.CreatedIDs["files"][0]))
	assert.Contains(t, p.Body, fmt.Sprintf("/courses/%d/assignments/%d", dst.ID, res.CreatedIDs["assignments"][0]))
	items, err := postgres.NewModuleItemRepository(g).ListByModuleID(ctx, res.CreatedIDs["modules"][0], lanePage)
	require.NoError(t, err)
	require.Len(t, items.Items, 4, "the file item now imports too")
	for _, it := range items.Items {
		if it.ContentType == "Attachment" {
			assert.Equal(t, res.CreatedIDs["files"][0], *it.ContentID)
		}
	}
}
