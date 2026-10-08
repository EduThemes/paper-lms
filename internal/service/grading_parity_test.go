package service_test

// Grading-lane parity regressions (Canvas comparison audit). Each test
// names the bug id it locks. All data is made up.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func latePolicy(perInterval float64, interval string, minPct float64) *models.LatePolicy {
	return &models.LatePolicy{
		LateSubmissionDeductionEnabled:      true,
		LateSubmissionDeduction:             perInterval,
		LateSubmissionInterval:              interval,
		LateSubmissionMinimumPercentEnabled: minPct > 0,
		LateSubmissionMinimumPercent:        minPct,
	}
}

// B020: Canvas LatePolicy#points_deducted — a percentage of points
// possible, capped so the floor never raises a low raw score.
func TestGradingParity_B020_LatePointsDeductedMatchesCanvas(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name          string
		policy        *models.LatePolicy
		score, pts    float64
		lateFor       time.Duration
		wantDeduction float64
	}{
		{"pct of possible, not of score", latePolicy(10, "day", 0), 8, 10, day, 1},
		{"floor never raises a low score", latePolicy(10, "day", 50), 2, 10, day, 0},
		{"floor caps the deduction", latePolicy(10, "day", 50), 8, 10, 10 * day, 3},
		{"partial interval rounds up", latePolicy(10, "day", 0), 10, 10, day + time.Minute, 2},
		{"hourly interval", latePolicy(5, "hour", 0), 100, 100, 3 * time.Hour, 15},
		{"never below zero", latePolicy(50, "day", 0), 3, 10, 5 * day, 3},
		{"on time", latePolicy(10, "day", 0), 8, 10, 0, 0},
		{"disabled", &models.LatePolicy{LateSubmissionDeduction: 10, LateSubmissionInterval: "day"}, 8, 10, day, 0},
	}
	for _, tc := range cases {
		got := service.LatePointsDeducted(tc.policy, tc.score, tc.pts, tc.lateFor)
		assert.InDelta(t, tc.wantDeduction, got, 1e-9, tc.name)
	}
}

type gradeHarness struct {
	svc            *service.SubmissionService
	submissionRepo *mocks.MockSubmissionRepository
	assignmentRepo *mocks.MockAssignmentRepository
	latePolicyRepo *mocks.MockLatePolicyRepository
	enrollmentRepo *mocks.MockEnrollmentRepository
}

func newGradeHarness(assignment *models.Assignment) *gradeHarness {
	h := &gradeHarness{
		submissionRepo: new(mocks.MockSubmissionRepository),
		assignmentRepo: new(mocks.MockAssignmentRepository),
		latePolicyRepo: new(mocks.MockLatePolicyRepository),
		enrollmentRepo: new(mocks.MockEnrollmentRepository),
	}
	h.assignmentRepo.On("FindByID", mock.Anything, assignment.ID, mock.Anything).Return(assignment, nil)
	courseRepo := new(mocks.MockCourseRepository)
	courseRepo.On("FindByID", mock.Anything, mock.Anything, mock.Anything).Return(&models.Course{ID: assignment.CourseID, AccountID: 1}, nil).Maybe()
	gpg := new(mocks.MockGradingPeriodGroupRepository)
	gpg.On("ListByAccountID", mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.PaginatedResult[models.GradingPeriodGroup]{}, nil).Maybe()
	h.svc = service.NewSubmissionService(h.submissionRepo, h.assignmentRepo, h.enrollmentRepo, h.latePolicyRepo,
		courseRepo, gpg, new(mocks.MockGradingPeriodRepository), nil)
	return h
}

// B021: re-entering the same grade on a late submission deducts once;
// the entered value is kept separately from the final score.
func TestGradingParity_B021_LatePenaltyDoesNotCompound(t *testing.T) {
	due := time.Now().Add(-72 * time.Hour)
	submitted := due.Add(20 * time.Hour) // 1 day late
	a := &models.Assignment{ID: 10, CourseID: 1, DueAt: &due, PointsPossible: ptrFloat64(10), GradingType: "points"}
	h := newGradeHarness(a)
	h.latePolicyRepo.On("FindByCourseID", mock.Anything, uint(1)).Return(latePolicy(10, "day", 0), nil)
	sub := &models.Submission{ID: 1, AssignmentID: 10, UserID: 20, SubmittedAt: &submitted, Late: true, WorkflowState: models.SubmissionSubmitted}
	h.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(1)).Return(sub, nil)
	h.submissionRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	for i := 0; i < 3; i++ {
		got, err := h.svc.Grade(context.Background(), 10, 20, 99, 1, "8")
		require.NoError(t, err)
		require.NotNil(t, got.Score)
		assert.InDelta(t, 7.0, *got.Score, 1e-9, "save #%d", i+1)
		require.NotNil(t, got.EnteredScore)
		assert.InDelta(t, 8.0, *got.EnteredScore, 1e-9)
		require.NotNil(t, got.PointsDeducted)
		assert.InDelta(t, 1.0, *got.PointsDeducted, 1e-9)
	}
}

// B022: an override that extends the due and lock dates lets the student
// submit after the base lock date, on time.
func TestGradingParity_B022_OverrideDatesHonoredOnSubmit(t *testing.T) {
	baseDue := time.Now().Add(-48 * time.Hour)
	baseLock := time.Now().Add(-24 * time.Hour)
	a := &models.Assignment{ID: 10, CourseID: 1, DueAt: &baseDue, LockAt: &baseLock,
		WorkflowState: models.AssignmentPublished, SubmissionTypes: "online_text_entry"}
	h := newGradeHarness(a)

	overrideDue := time.Now().Add(24 * time.Hour)
	overrideLock := time.Now().Add(48 * time.Hour)
	overrideRepo := new(mocks.MockAssignmentOverrideRepository)
	studentRepo := new(mocks.MockAssignmentOverrideStudentRepository)
	studentRepo.On("ListByUserAndAssignment", mock.Anything, uint(20), uint(10)).
		Return([]models.AssignmentOverrideStudent{{AssignmentOverrideID: 3, UserID: 20, AssignmentID: 10}}, nil)
	overrideRepo.On("FindByID", mock.Anything, uint(3)).
		Return(&models.AssignmentOverride{ID: 3, AssignmentID: 10, DueAt: &overrideDue, LockAt: &overrideLock, WorkflowState: "active"}, nil)
	h.enrollmentRepo.On("ListByUserID", mock.Anything, uint(20), uint(0)).Return([]models.Enrollment{}, nil)
	h.svc.SetOverrideService(service.NewOverrideService(overrideRepo, studentRepo, h.enrollmentRepo, new(mocks.MockSectionRepository)))

	h.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(0)).Return(nil, errors.New("not found"))
	h.submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).Return(nil)

	body := "work"
	sub := &models.Submission{AssignmentID: 10, UserID: 20, SubmissionType: ptrString("online_text_entry"), Body: &body}
	require.NoError(t, h.svc.Submit(context.Background(), sub, 1, 1))
	assert.False(t, sub.Late, "on time against the override due date")

	// Without the override the same submission is locked.
	plain := newGradeHarness(a)
	sub2 := &models.Submission{AssignmentID: 10, UserID: 20, SubmissionType: ptrString("online_text_entry"), Body: &body}
	assert.ErrorIs(t, plain.svc.Submit(context.Background(), sub2, 1, 1), service.ErrSubmissionLocked)
}

// B121: every grade write reaches the grade-change audit log with the
// previous and new values.
func TestGradingParity_B121_GradeChangeLogged(t *testing.T) {
	a := &models.Assignment{ID: 10, CourseID: 1, PointsPossible: ptrFloat64(10)}
	h := newGradeHarness(a)
	old := 4.0
	oldGrade := "4"
	h.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(1)).
		Return(&models.Submission{ID: 1, AssignmentID: 10, UserID: 20, Score: &old, Grade: &oldGrade}, nil)
	h.submissionRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	var logged []service.GradeChange
	h.svc.SetGradeChangeLogger(func(_ context.Context, ch service.GradeChange) { logged = append(logged, ch) })

	_, err := h.svc.Grade(context.Background(), 10, 20, 99, 1, "9")
	require.NoError(t, err)
	require.Len(t, logged, 1)
	assert.Equal(t, uint(1), logged[0].CourseID)
	assert.Equal(t, uint(99), logged[0].GraderID)
	assert.Equal(t, "4", logged[0].OldGrade)
	assert.Equal(t, "9", logged[0].NewGrade)
	require.NotNil(t, logged[0].OldScore)
	assert.Equal(t, 4.0, *logged[0].OldScore)
}

// B115 / B206: excused work and unpublished assignments don't count;
// a 0-point assignment's score is extra credit.
func TestGradingParity_B115_GradeTotalsExcludeExcusedAndUnpublished(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{
			{ID: 1, CourseID: 1, PointsPossible: ptrFloat64(100), WorkflowState: models.AssignmentPublished},
			{ID: 2, CourseID: 1, PointsPossible: ptrFloat64(100), WorkflowState: models.AssignmentPublished}, // excused
			{ID: 3, CourseID: 1, PointsPossible: ptrFloat64(100), WorkflowState: models.AssignmentUnpublished},
			{ID: 4, CourseID: 1, PointsPossible: ptrFloat64(0), WorkflowState: models.AssignmentPublished}, // extra credit
		},
	}, nil)
	submissionRepo.On("ListByUserAndCourse", mock.Anything, uint(10), uint(1)).Return([]models.Submission{
		{AssignmentID: 1, UserID: 10, Score: ptrFloat64(80)},
		{AssignmentID: 2, UserID: 10, Score: ptrFloat64(0), Excused: true},
		{AssignmentID: 4, UserID: 10, Score: ptrFloat64(5)},
	}, nil)
	svc := service.NewGradingService(submissionRepo, assignmentRepo, new(mocks.MockAssignmentGroupRepository),
		new(mocks.MockEnrollmentRepository), unweightedCourseRepo(1), nil)

	grade, err := svc.GetStudentGrade(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 85.0, grade.CurrentScore)
	assert.Equal(t, 85.0, grade.FinalScore)
}

// B117: reassigning keeps completed reviews, and graded students stay in
// the reviewer pool.
func TestGradingParity_B117_ReassignKeepsCompletedReviews(t *testing.T) {
	prRepo := new(mocks.MockPeerReviewRepository)
	subRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	enrollmentRepo.On("ListByCourseID", mock.Anything, uint(1), uint(0), mock.Anything).Return(&repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{
			{UserID: 11, Type: "StudentEnrollment"},
			{UserID: 12, Type: "StudentEnrollment"},
		},
	}, nil)
	now := time.Now()
	subRepo.On("FindByAssignmentAndUser", mock.Anything, uint(5), uint(11), uint(1)).
		Return(&models.Submission{ID: 111, UserID: 11, SubmittedAt: &now, WorkflowState: models.SubmissionGraded}, nil)
	subRepo.On("FindByAssignmentAndUser", mock.Anything, uint(5), uint(12), uint(1)).
		Return(&models.Submission{ID: 112, UserID: 12, SubmittedAt: &now, WorkflowState: models.SubmissionSubmitted}, nil)
	prRepo.On("ListByAssignment", mock.Anything, uint(5), uint(1)).Return([]models.PeerReview{
		{ID: 1, AssignmentID: 5, ReviewerID: 11, RevieweeID: 12, WorkflowState: "completed"},
	}, nil)
	prRepo.On("DeleteIncompleteByAssignment", mock.Anything, uint(5), uint(1)).Return(nil).Once()
	prRepo.On("Create", mock.Anything, mock.MatchedBy(func(pr *models.PeerReview) bool {
		return pr.ReviewerID == 12 && pr.RevieweeID == 11
	})).Return(nil).Once()

	svc := service.NewPeerReviewService(prRepo, subRepo, enrollmentRepo)
	reviews, err := svc.AssignPeerReviews(context.Background(), 1, 5, 1, 1)
	require.NoError(t, err)
	assert.Len(t, reviews, 2, "the completed review is kept and the other student gets one")
	prRepo.AssertExpectations(t)
	prRepo.AssertNotCalled(t, "DeleteByAssignment", mock.Anything, mock.Anything, mock.Anything)
}

// B120: typed (string) rubric points still produce a score.
func TestGradingParity_B120_StringRubricPointsScore(t *testing.T) {
	assessRepo := new(mocks.MockRubricAssessmentRepository)
	assessRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.RubricAssessment")).Return(nil)
	svc := service.NewRubricService(new(mocks.MockRubricRepository), new(mocks.MockRubricAssociationRepository), assessRepo)

	a := &models.RubricAssessment{ID: 1, Data: `{"c1":{"points":"4"},"c2":{"points":2.5},"c3":{"points":""}}`, Score: ptrFloat64(99)}
	require.NoError(t, svc.UpdateAssessment(context.Background(), a))
	require.NotNil(t, a.Score)
	assert.Equal(t, 6.5, *a.Score)
}

// B110: the observer average counts automatically-posted grades and the
// recent feed never shows a hidden (unposted) grade.
func TestGradingParity_B110_ObserverSeesOnlyPostedGrades(t *testing.T) {
	enrollmentRepo := new(mocks.MockEnrollmentRepository)
	courseRepo := new(mocks.MockCourseRepository)
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	quizRepo := new(mocks.MockQuizRepository)
	pageRepo := new(mocks.MockPageRepository)

	child := uint(20)
	enrollmentRepo.On("ListByUserID", mock.Anything, uint(30), uint(0)).Return([]models.Enrollment{
		{UserID: 30, CourseID: 1, Type: "ObserverEnrollment", WorkflowState: "active", AssociatedUserID: &child},
	}, nil)
	enrollmentRepo.On("ListByUserID", mock.Anything, child, uint(0)).Return([]models.Enrollment{
		{UserID: child, CourseID: 1, Type: "StudentEnrollment", WorkflowState: "active"},
	}, nil)
	courseRepo.On("FindByID", mock.Anything, uint(1), uint(0)).Return(&models.Course{ID: 1, Name: "Biology"}, nil)
	assignmentRepo.On("ListByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.Assignment]{
		Items: []models.Assignment{
			{ID: 1, CourseID: 1, Name: "Posted", PointsPossible: ptrFloat64(10), WorkflowState: models.AssignmentPublished, Published: true},
			{ID: 2, CourseID: 1, Name: "Hidden", PointsPossible: ptrFloat64(10), WorkflowState: models.AssignmentPublished, Published: true, PostPolicy: "manual"},
		},
	}, nil)
	now := time.Now()
	submissionRepo.On("ListByUserAndCourse", mock.Anything, child, uint(1)).Return([]models.Submission{
		{ID: 1, AssignmentID: 1, UserID: child, Score: ptrFloat64(8), GradedAt: &now, PostedAt: &now, SubmittedAt: &now, WorkflowState: models.SubmissionGraded},
		{ID: 2, AssignmentID: 2, UserID: child, Score: ptrFloat64(2), GradedAt: &now, SubmittedAt: &now, WorkflowState: models.SubmissionGraded},
	}, nil)
	quizRepo.On("ListByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.Quiz]{}, nil)
	pageRepo.On("ListByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{}, nil).Maybe()
	pageRepo.On("ListPublishedByCourseID", mock.Anything, uint(1), mock.Anything).Return(&repository.PaginatedResult[models.WikiPage]{}, nil).Maybe()

	svc := service.NewObserverService(enrollmentRepo, courseRepo, new(mocks.MockUserRepository))
	svc.SetOverviewDeps(assignmentRepo, submissionRepo, quizRepo, emptyAnnouncements{}, pageRepo)
	ov, err := svc.GetChildOverview(context.Background(), 30, child)
	require.NoError(t, err)
	require.Len(t, ov.Courses, 1)
	require.NotNil(t, ov.Courses[0].CurrentGrade)
	assert.InDelta(t, 80.0, *ov.Courses[0].CurrentGrade, 1e-9)
	require.Len(t, ov.RecentGrades, 1)
	assert.Equal(t, uint(1), ov.RecentGrades[0].AssignmentID)
}

// B113 (group assignments): a group grade is applied to every member, so
// clearing it clears every member's grade too.
func TestGradingParity_B113_ClearingGroupGradeClearsMembers(t *testing.T) {
	cat := uint(4)
	a := &models.Assignment{ID: 10, CourseID: 1, PointsPossible: ptrFloat64(10), GroupCategoryID: &cat}
	h := newGradeHarness(a)
	groups := new(mocks.MockGroupMembershipRepository)
	groups.On("FindUserGroupInCategory", mock.Anything, uint(20), uint(4)).Return(&models.Group{ID: 8}, nil)
	groups.On("ListByGroupID", mock.Anything, uint(8), mock.Anything).Return(&repository.PaginatedResult[models.GroupMembership]{
		Items: []models.GroupMembership{{UserID: 20, WorkflowState: "accepted"}, {UserID: 21, WorkflowState: "accepted"}},
	}, nil)
	courseRepo := new(mocks.MockCourseRepository)
	courseRepo.On("FindByID", mock.Anything, mock.Anything, mock.Anything).Return(&models.Course{ID: 1, AccountID: 1}, nil).Maybe()
	gpg := new(mocks.MockGradingPeriodGroupRepository)
	gpg.On("ListByAccountID", mock.Anything, mock.Anything, mock.Anything).
		Return(&repository.PaginatedResult[models.GradingPeriodGroup]{}, nil).Maybe()
	svc := service.NewSubmissionService(h.submissionRepo, h.assignmentRepo, h.enrollmentRepo, h.latePolicyRepo,
		courseRepo, gpg, new(mocks.MockGradingPeriodRepository), groups)

	h.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(20), uint(1)).
		Return(&models.Submission{ID: 1, AssignmentID: 10, UserID: 20, Score: ptrFloat64(8), Grade: ptrString("8"), WorkflowState: models.SubmissionGraded}, nil)
	member := &models.Submission{ID: 2, AssignmentID: 10, UserID: 21, Score: ptrFloat64(8), Grade: ptrString("8"), EnteredScore: ptrFloat64(8), WorkflowState: models.SubmissionGraded}
	h.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(10), uint(21), uint(1)).Return(member, nil)
	h.submissionRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.Grade(context.Background(), 10, 20, 99, 1, "")
	require.NoError(t, err)
	assert.Nil(t, member.Score, "member's group grade must be cleared")
	assert.Nil(t, member.EnteredScore)
	assert.Equal(t, models.SubmissionUnsubmitted, member.WorkflowState)
}

type emptyAnnouncements struct{}

func (emptyAnnouncements) ListByCourseID(context.Context, uint, repository.PaginationParams) (*repository.PaginatedResult[models.Announcement], error) {
	return &repository.PaginatedResult[models.Announcement]{}, nil
}

// F14/F22: OnGraded callbacks (mastery paths, gamification, outcome
// results) see a grade only once the student can: a grade hidden by a
// manual post policy does not fire; posting fires it (FirePosted); an
// automatic post policy fires at grading time.
func TestGradingParity_OnGradedFiresOnlyForVisibleGrades(t *testing.T) {
	collect := func(svc *service.SubmissionService) chan uint {
		ch := make(chan uint, 4)
		svc.OnGraded(func(_ context.Context, id uint) { ch <- id })
		return ch
	}
	waitFor := func(ch chan uint) (uint, bool) {
		select {
		case id := <-ch:
			return id, true
		case <-time.After(300 * time.Millisecond):
			return 0, false
		}
	}

	manual := &models.Assignment{ID: 30, CourseID: 1, PointsPossible: ptrFloat64(100), GradingType: "points", PostPolicy: "manual"}
	h := newGradeHarness(manual)
	h.latePolicyRepo.On("FindByCourseID", mock.Anything, uint(1)).Return(nil, errors.New("none")).Maybe()
	h.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(30), uint(20), uint(1)).
		Return(&models.Submission{ID: 7, AssignmentID: 30, UserID: 20, WorkflowState: models.SubmissionSubmitted}, nil)
	h.submissionRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	ch := collect(h.svc)

	got, err := h.svc.Grade(context.Background(), 30, 20, 99, 1, "45")
	require.NoError(t, err)
	assert.Nil(t, got.PostedAt, "manual post policy keeps the grade hidden")
	_, fired := waitFor(ch)
	assert.False(t, fired, "a hidden grade must not fire OnGraded")

	h.svc.FirePosted(got.ID)
	id, fired := waitFor(ch)
	assert.True(t, fired, "posting fires OnGraded")
	assert.Equal(t, uint(7), id)

	auto := &models.Assignment{ID: 31, CourseID: 1, PointsPossible: ptrFloat64(100), GradingType: "points"}
	h2 := newGradeHarness(auto)
	h2.latePolicyRepo.On("FindByCourseID", mock.Anything, uint(1)).Return(nil, errors.New("none")).Maybe()
	h2.submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(31), uint(20), uint(1)).
		Return(&models.Submission{ID: 8, AssignmentID: 31, UserID: 20, WorkflowState: models.SubmissionSubmitted}, nil)
	h2.submissionRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	ch2 := collect(h2.svc)
	_, err = h2.svc.Grade(context.Background(), 31, 20, 99, 1, "90")
	require.NoError(t, err)
	id, fired = waitFor(ch2)
	assert.True(t, fired)
	assert.Equal(t, uint(8), id)
}
