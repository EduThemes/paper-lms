package service_test

// Content-lane service regressions. The blueprint and Commons tests run
// against a migrated Postgres (freshDB, skipped without one); the pacing
// override test uses mocks.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func laneSeedCourse(t *testing.T, g *gorm.DB, accountID uint, code string) *models.Course {
	t.Helper()
	require.NoError(t, g.Exec(
		`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500) ON CONFLICT (id) DO NOTHING`,
		accountID, fmt.Sprintf("District %d", accountID)).Error)
	c := &models.Course{AccountID: accountID, Name: code, CourseCode: code, WorkflowState: models.CourseAvailable}
	require.NoError(t, postgres.NewCourseRepository(g).Create(context.Background(), c))
	return c
}

func newLaneBlueprintService(g *gorm.DB) *service.BlueprintService {
	svc := service.NewBlueprintService(
		postgres.NewBlueprintTemplateRepository(g),
		postgres.NewBlueprintSubscriptionRepository(g),
		postgres.NewBlueprintMigrationRepository(g),
		postgres.NewModuleRepository(g),
		postgres.NewModuleItemRepository(g),
		postgres.NewAssignmentRepository(g),
		postgres.NewPageRepository(g),
		postgres.NewQuizRepository(g),
		postgres.NewQuizQuestionRepository(g),
		postgres.NewDiscussionTopicRepository(g),
	)
	svc.SetDeps(postgres.NewCourseRepository(g), postgres.NewEnrollmentRepository(g), postgres.NewBlueprintContentTagRepository(g))
	return svc
}

// B041: a second sync pushes blueprint edits (including a rename) to the
// existing child copy instead of skipping it, never duplicates it, and
// keeps a teacher's local edit in the child when content is unlocked.
func TestBlueprintSync_PropagatesEdits(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	bp := laneSeedCourse(t, g, 1, "BP-101")
	child := laneSeedCourse(t, g, 1, "SEC-1")
	assignRepo := postgres.NewAssignmentRepository(g)
	pageRepo := postgres.NewPageRepository(g)
	svc := newLaneBlueprintService(g)

	pts := 10.0
	essay := &models.Assignment{CourseID: bp.ID, Name: "Essay 1", Description: "Write 500 wrods", PointsPossible: &pts, WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, assignRepo.Create(ctx, essay))
	page := &models.WikiPage{CourseID: bp.ID, Title: "Welcome", Body: "<p>Hello</p>", WorkflowState: "active"}
	require.NoError(t, pageRepo.Create(ctx, page))

	tmpl, err := svc.GetOrCreateTemplate(ctx, bp.ID)
	require.NoError(t, err)
	require.NoError(t, svc.ModifyAssociations(ctx, tmpl, service.BlueprintCaller{UserID: 1, AccountID: 1, IsAdmin: true}, []uint{child.ID}, nil))

	_, err = svc.TriggerSync(ctx, tmpl.ID, 1, "initial")
	require.NoError(t, err)
	childAssignments, err := assignRepo.ListByCourseID(ctx, child.ID, repository.PaginationParams{Page: 1, PerPage: 100})
	require.NoError(t, err)
	require.Len(t, childAssignments.Items, 1)
	copyID := childAssignments.Items[0].ID

	// The teacher edits the child's page locally (a downstream change).
	childPages, err := pageRepo.ListByCourseID(ctx, child.ID, repository.PaginationParams{Page: 1, PerPage: 100})
	require.NoError(t, err)
	require.Len(t, childPages.Items, 1)
	time.Sleep(1100 * time.Millisecond)
	localPage := childPages.Items[0]
	localPage.Body = "<p>Local section note</p>"
	require.NoError(t, pageRepo.Update(ctx, &localPage))

	// Blueprint edits: fix the typo, move the due date, rename.
	time.Sleep(1100 * time.Millisecond)
	due := time.Date(2026, 11, 2, 23, 59, 0, 0, time.UTC)
	essay.Description = "Write 500 words"
	essay.DueAt = &due
	essay.Name = "Essay 1: Argument"
	require.NoError(t, assignRepo.Update(ctx, essay))
	page.Body = "<p>Hello, class</p>"
	require.NoError(t, pageRepo.Update(ctx, page))

	changes, err := svc.GetUnsyncedChanges(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, changes)

	_, err = svc.TriggerSync(ctx, tmpl.ID, 1, "fixes")
	require.NoError(t, err)

	childAssignments, err = assignRepo.ListByCourseID(ctx, child.ID, repository.PaginationParams{Page: 1, PerPage: 100})
	require.NoError(t, err)
	require.Len(t, childAssignments.Items, 1, "a rename must update the copy, not duplicate it")
	got := childAssignments.Items[0]
	assert.Equal(t, copyID, got.ID)
	assert.Equal(t, "Essay 1: Argument", got.Name)
	assert.Equal(t, "Write 500 words", got.Description)
	require.NotNil(t, got.DueAt)
	assert.True(t, got.DueAt.Equal(due))

	// Unlocked content: the child's local page edit survives.
	gotPage, err := pageRepo.FindByID(ctx, localPage.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, "<p>Local section note</p>", gotPage.Body)

	// After the sync nothing is pending.
	changes, err = svc.GetUnsyncedChanges(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Empty(t, changes)
}

// B037 (service): only same-account courses the caller teaches can be
// associated.
func TestBlueprintModifyAssociations_RejectsOtherTenant(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	bp := laneSeedCourse(t, g, 1, "BP-101")
	foreign := laneSeedCourse(t, g, 2, "OTHER-1")
	svc := newLaneBlueprintService(g)
	tmpl, err := svc.GetOrCreateTemplate(ctx, bp.ID)
	require.NoError(t, err)

	err = svc.ModifyAssociations(ctx, tmpl, service.BlueprintCaller{UserID: 1, AccountID: 1, IsAdmin: true}, []uint{foreign.ID}, nil)
	assert.ErrorIs(t, err, service.ErrBlueprintInvalidAssociation)
	subs, err := svc.ListAssociatedCourses(ctx, tmpl.ID, repository.PaginationParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.Zero(t, subs.TotalCount)
}

// B145: a Commons quiz import brings its questions, and a module import
// brings its items wired to freshly imported content.
func TestCommonsImport_CopiesQuizQuestionsAndModuleItems(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	src := laneSeedCourse(t, g, 1, "SRC-1")
	dst := laneSeedCourse(t, g, 1, "DST-1")

	quizRepo := postgres.NewQuizRepository(g)
	qqRepo := postgres.NewQuizQuestionRepository(g)
	moduleRepo := postgres.NewModuleRepository(g)
	itemRepo := postgres.NewModuleItemRepository(g)
	assignRepo := postgres.NewAssignmentRepository(g)
	svc := service.NewCommonsService(
		postgres.NewSharedContentRepository(g), postgres.NewCourseRepository(g), assignRepo,
		postgres.NewPageRepository(g), quizRepo, moduleRepo, postgres.NewDiscussionTopicRepository(g),
	)
	svc.SetContentDeps(qqRepo, itemRepo)

	quiz := &models.Quiz{CourseID: src.ID, Title: "Cells quiz", WorkflowState: "available"}
	require.NoError(t, quizRepo.Create(ctx, quiz))
	for i, text := range []string{"What is a cell?", "Name an organelle."} {
		require.NoError(t, qqRepo.Create(ctx, &models.QuizQuestion{QuizID: quiz.ID, Position: i + 1, QuestionType: "essay_question", QuestionText: text, Answers: "[]", WorkflowState: "active"}))
	}

	shared, err := svc.Publish(ctx, 1, src.ID, 1, service.CommonsPublishOptions{ResourceType: "quiz", ResourceID: quiz.ID, Title: "Cells quiz"})
	require.NoError(t, err)
	res, err := svc.Import(ctx, 1, dst.ID, shared.ID, 1)
	require.NoError(t, err)
	require.Len(t, res.CreatedIDs["quizzes"], 1)
	qs, err := qqRepo.ListByQuizID(ctx, res.CreatedIDs["quizzes"][0], repository.PaginationParams{Page: 1, PerPage: 50})
	require.NoError(t, err)
	assert.Len(t, qs.Items, 2, "imported quiz must keep its questions")

	// Module with an assignment item.
	essay := &models.Assignment{CourseID: src.ID, Name: "Lab report", WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, assignRepo.Create(ctx, essay))
	mod := &models.ContextModule{CourseID: src.ID, Name: "Unit 1", Position: 1, WorkflowState: "active"}
	require.NoError(t, moduleRepo.Create(ctx, mod))
	essayID := essay.ID
	require.NoError(t, itemRepo.Create(ctx, &models.ContentTag{ContextModuleID: mod.ID, ContentType: "Assignment", ContentID: &essayID, Title: "Lab report", Position: 1, WorkflowState: "active"}))
	require.NoError(t, itemRepo.Create(ctx, &models.ContentTag{ContextModuleID: mod.ID, ContentType: "ContextModuleSubHeader", Title: "Readings", Position: 2, WorkflowState: "active"}))

	shared, err = svc.Publish(ctx, 1, src.ID, 1, service.CommonsPublishOptions{ResourceType: "module", ResourceID: mod.ID, Title: "Unit 1"})
	require.NoError(t, err)
	res, err = svc.Import(ctx, 1, dst.ID, shared.ID, 1)
	require.NoError(t, err)
	require.Len(t, res.CreatedIDs["modules"], 1)
	require.Len(t, res.CreatedIDs["assignments"], 1)
	items, err := itemRepo.ListByModuleID(ctx, res.CreatedIDs["modules"][0], repository.PaginationParams{Page: 1, PerPage: 50})
	require.NoError(t, err)
	require.Len(t, items.Items, 2, "imported module must keep its items")
	require.NotNil(t, items.Items[0].ContentID)
	assert.Equal(t, res.CreatedIDs["assignments"][0], *items.Items[0].ContentID, "item must point at the imported copy, not the source")

	// B036/B038 (service): another tenant's course is neither a valid
	// import target nor publish source.
	foreign := laneSeedCourse(t, g, 2, "OTHER-1")
	_, err = svc.Import(ctx, 1, foreign.ID, shared.ID, 1)
	assert.ErrorIs(t, err, service.ErrCommonsNotFound)
	_, err = svc.Publish(ctx, 1, src.ID, 1, service.CommonsPublishOptions{ResourceType: "quiz", ResourceID: quiz.ID + 999, Title: "x"})
	assert.ErrorIs(t, err, service.ErrCommonsNotFound)
}

// B144: publishing a student pace writes a per-student override and
// leaves the shared assignment due date alone; the schedule starts at
// the course start date, not the publish moment.
func TestCoursePacePublish_StudentPaceUsesOverride(t *testing.T) {
	paces := new(mocks.MockCoursePaceRepository)
	paceItems := new(mocks.MockCoursePaceModuleItemRepository)
	items := new(mocks.MockModuleItemRepository)
	modules := new(mocks.MockModuleRepository)
	assignments := new(mocks.MockAssignmentRepository)
	overrides := new(mocks.MockAssignmentOverrideRepository)
	overrideStudents := new(mocks.MockAssignmentOverrideStudentRepository)
	courses := new(mocks.MockCourseRepository)

	svc := service.NewCoursePaceService(paces, paceItems, items, assignments)
	svc.SetDeps(service.CoursePaceDeps{ModuleRepo: modules, OverrideRepo: overrides, OverrideStudentRepo: overrideStudents, CourseRepo: courses})

	student := uint(55)
	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC) // a Tuesday
	paces.On("FindByID", mock.Anything, uint(7), uint(1)).Return(&models.CoursePace{ID: 7, CourseID: 10, UserID: &student}, nil)
	paces.On("Update", mock.Anything, mock.Anything).Return(nil)
	courses.On("FindByID", mock.Anything, uint(10), uint(0)).Return(&models.Course{ID: 10, AccountID: 1, StartAt: &start}, nil)
	paceItems.On("ListByPaceID", mock.Anything, uint(7)).Return([]models.CoursePaceModuleItem{{CoursePaceID: 7, ModuleItemID: 300, Duration: 3}}, nil)
	contentID := uint(900)
	items.On("FindByID", mock.Anything, uint(300)).Return(&models.ContentTag{ID: 300, ContextModuleID: 40, ContentType: "Assignment", ContentID: &contentID, WorkflowState: "active"}, nil)
	modules.On("FindByID", mock.Anything, uint(40), uint(0)).Return(&models.ContextModule{ID: 40, CourseID: 10, WorkflowState: "active"}, nil)
	assignments.On("FindByID", mock.Anything, uint(900), uint(0)).Return(&models.Assignment{ID: 900, CourseID: 10, Name: "Essay"}, nil)
	overrideStudents.On("ListByUserAndAssignment", mock.Anything, student, uint(900)).Return([]models.AssignmentOverrideStudent{}, nil)

	var created *models.AssignmentOverride
	overrides.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created = args.Get(1).(*models.AssignmentOverride)
		created.ID = 77
	}).Return(nil)
	overrideStudents.On("Create", mock.Anything, mock.MatchedBy(func(s *models.AssignmentOverrideStudent) bool {
		return s.AssignmentOverrideID == 77 && s.UserID == student && s.AssignmentID == 900
	})).Return(nil)

	_, err := svc.PublishPace(context.Background(), 7, 1)
	require.NoError(t, err)

	assignments.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	require.NotNil(t, created)
	require.NotNil(t, created.DueAt)
	assert.Equal(t, service.CoursePacingOverrideTitle, created.Title)
	assert.True(t, created.DueAt.Equal(start.AddDate(0, 0, 3)), "3 weekday-days after the course start, got %v", created.DueAt)
	overrideStudents.AssertExpectations(t)
}

// B039 (service): applyPaceDates never follows a module item into
// another course.
func TestCoursePacePublish_IgnoresForeignModuleItems(t *testing.T) {
	paces := new(mocks.MockCoursePaceRepository)
	paceItems := new(mocks.MockCoursePaceModuleItemRepository)
	items := new(mocks.MockModuleItemRepository)
	modules := new(mocks.MockModuleRepository)
	assignments := new(mocks.MockAssignmentRepository)

	svc := service.NewCoursePaceService(paces, paceItems, items, assignments)
	svc.SetDeps(service.CoursePaceDeps{ModuleRepo: modules})

	paces.On("FindByID", mock.Anything, uint(7), uint(1)).Return(&models.CoursePace{ID: 7, CourseID: 10, CreatedAt: time.Now()}, nil)
	paces.On("Update", mock.Anything, mock.Anything).Return(nil)
	paceItems.On("ListByPaceID", mock.Anything, uint(7)).Return([]models.CoursePaceModuleItem{{CoursePaceID: 7, ModuleItemID: 300, Duration: 1}}, nil)
	contentID := uint(900)
	items.On("FindByID", mock.Anything, uint(300)).Return(&models.ContentTag{ID: 300, ContextModuleID: 40, ContentType: "Assignment", ContentID: &contentID, WorkflowState: "active"}, nil)
	modules.On("FindByID", mock.Anything, uint(40), uint(0)).Return(&models.ContextModule{ID: 40, CourseID: 99, WorkflowState: "active"}, nil)

	_, err := svc.PublishPace(context.Background(), 7, 1)
	require.NoError(t, err)
	assignments.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything, mock.Anything)
	assignments.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// F13/F23: per-attempt bank-draw copies (question_bank_entry_id) in the
// blueprint quiz are not synced as fixed questions, don't count as
// unsynced changes, and the child's own copies are left alone.
func TestBlueprintSync_SkipsBankDrawCopies(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	bp := laneSeedCourse(t, g, 1, "BP-201")
	child := laneSeedCourse(t, g, 1, "SEC-2")
	quizRepo := postgres.NewQuizRepository(g)
	qqRepo := postgres.NewQuizQuestionRepository(g)
	svc := newLaneBlueprintService(g)

	quiz := &models.Quiz{CourseID: bp.ID, Title: "Unit quiz", QuizType: "assignment", WorkflowState: "available"}
	require.NoError(t, quizRepo.Create(ctx, quiz))
	require.NoError(t, qqRepo.Create(ctx, &models.QuizQuestion{QuizID: quiz.ID, Position: 1, QuestionType: "essay_question", QuestionText: "Explain osmosis", WorkflowState: "active"}))

	tmpl, err := svc.GetOrCreateTemplate(ctx, bp.ID)
	require.NoError(t, err)
	require.NoError(t, svc.ModifyAssociations(ctx, tmpl, service.BlueprintCaller{UserID: 1, AccountID: 1, IsAdmin: true}, []uint{child.ID}, nil))
	_, err = svc.TriggerSync(ctx, tmpl.ID, 1, "initial")
	require.NoError(t, err)

	childQuizzes, err := quizRepo.ListByCourseID(ctx, child.ID, repository.PaginationParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	require.Len(t, childQuizzes.Items, 1)
	childQuizID := childQuizzes.Items[0].ID

	// A student attempt in the child materializes a bank draw there, and
	// one in the blueprint materializes another draw in the source quiz.
	childEntry, srcEntry := uint(9001), uint(9002)
	require.NoError(t, qqRepo.Create(ctx, &models.QuizQuestion{QuizID: childQuizID, Position: 2, QuestionType: "multiple_choice_question", QuestionText: "Child draw", WorkflowState: "active", QuestionBankEntryID: &childEntry}))
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, qqRepo.Create(ctx, &models.QuizQuestion{QuizID: quiz.ID, Position: 2, QuestionType: "multiple_choice_question", QuestionText: "Source draw", WorkflowState: "active", QuestionBankEntryID: &srcEntry}))

	changes, err := svc.GetUnsyncedChanges(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Empty(t, changes, "a student's bank draw is not an authoring change")

	// Force a re-sync of the quiz by editing its authored question.
	time.Sleep(1100 * time.Millisecond)
	quiz.Description = "Updated"
	require.NoError(t, quizRepo.Update(ctx, quiz))
	_, err = svc.TriggerSync(ctx, tmpl.ID, 1, "edit")
	require.NoError(t, err)

	got, err := qqRepo.ListByQuizID(ctx, childQuizID, repository.PaginationParams{Page: 1, PerPage: 100})
	require.NoError(t, err)
	texts := map[string]bool{}
	for _, q := range got.Items {
		texts[q.QuestionText] = true
	}
	assert.True(t, texts["Explain osmosis"])
	assert.True(t, texts["Child draw"], "the child's own bank copy must not be overwritten or deleted")
	assert.False(t, texts["Source draw"], "the source's bank copy must not become a fixed child question")
	assert.Len(t, got.Items, 2)
}
