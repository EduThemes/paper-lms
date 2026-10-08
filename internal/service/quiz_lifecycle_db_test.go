package service_test

// Real-Postgres tests for the quiz lifecycle (quiz-engine lane):
//   G041 graded quiz ↔ online_quiz assignment, scores reach the gradebook
//        through late / post policy
//   G042 manual grading (scores, comments, fudge points, pending_review)
//   G044 scoring policy across attempts
//   G046 student override dates on the linked assignment
//   G047 extensions (extra attempts / time, manual unlock, extend attempt)
//   G048 overdue auto-submit, end_at capped at lock_at
//   G053 regrade
//   G057 attempt event log
//
// Skipped when PARITY_DB_URL / DATABASE_URL is unset (freshDB).

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

type lifecycleFixture struct {
	g        *gorm.DB
	svc      *service.QuizService
	subs     *service.SubmissionService
	courseID uint
	groupID  uint
	teacher  uint
	students []uint
	deps     service.QuizLifecycleDeps
}

func newLifecycleFixture(t *testing.T, nStudents int) *lifecycleFixture {
	t.Helper()
	g, cleanup := freshDB(t)
	t.Cleanup(cleanup)

	require.NoError(t, g.Exec(`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		VALUES (1, 'Account 1', 'active', 'off', 'en', 'higher_ed', 500) ON CONFLICT (id) DO NOTHING`).Error)
	course := &models.Course{AccountID: 1, Name: "Biology", CourseCode: "BIO-1", WorkflowState: "available"}
	require.NoError(t, g.Create(course).Error)
	group := &models.AssignmentGroup{CourseID: course.ID, Name: "Quizzes", Position: 1, WorkflowState: "available"}
	require.NoError(t, g.Create(group).Error)

	teacher := &models.User{AccountID: 1, Name: "Terry Teacher", LoginID: "terry.t", Email: "terry.t@example.com", Role: "user"}
	require.NoError(t, g.Create(teacher).Error)
	f := &lifecycleFixture{g: g, courseID: course.ID, groupID: group.ID, teacher: teacher.ID}
	for i := 0; i < nStudents; i++ {
		u := &models.User{AccountID: 1, Name: fmt.Sprintf("Student %d", i+1), LoginID: fmt.Sprintf("stu%d", i+1), Email: fmt.Sprintf("stu%d@example.com", i+1), Role: "user"}
		require.NoError(t, g.Create(u).Error)
		require.NoError(t, g.Create(&models.Enrollment{UserID: u.ID, CourseID: course.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active"}).Error)
		f.students = append(f.students, u.ID)
	}

	assignmentRepo := postgres.NewAssignmentRepository(g)
	enrollmentRepo := postgres.NewEnrollmentRepository(g)
	f.subs = service.NewSubmissionService(postgres.NewSubmissionRepository(g), assignmentRepo, enrollmentRepo,
		postgres.NewLatePolicyRepository(g), postgres.NewCourseRepository(g),
		postgres.NewGradingPeriodGroupRepository(g), postgres.NewGradingPeriodRepository(g), postgres.NewGroupMembershipRepository(g))
	f.subs.SetOverrideService(service.NewOverrideService(postgres.NewAssignmentOverrideRepository(g),
		postgres.NewAssignmentOverrideStudentRepository(g), enrollmentRepo, postgres.NewSectionRepository(g)))

	submissionRepo := postgres.NewQuizSubmissionRepository(g)
	f.deps = service.QuizLifecycleDeps{
		Assignments:      assignmentRepo,
		AssignmentGroups: postgres.NewAssignmentGroupRepository(g),
		Gradebook:        f.subs,
		Extensions:       postgres.NewQuizExtensionRepository(g),
		Events:           postgres.NewQuizSubmissionEventRepository(g),
		Regrades:         postgres.NewQuizQuestionRegradeRepository(g),
		Overdue:          submissionRepo,
		Enrollments:      enrollmentRepo,
		Unlinked:         postgres.NewQuizRepository(g),
	}
	f.svc = f.newService(f.deps)
	return f
}

// newService builds a QuizService over the fixture's database with the
// given lifecycle dependencies.
func (f *lifecycleFixture) newService(deps service.QuizLifecycleDeps) *service.QuizService {
	return service.NewQuizService(
		postgres.NewQuizRepository(f.g),
		postgres.NewQuizQuestionRepository(f.g),
		postgres.NewQuizSubmissionRepository(f.g),
		postgres.NewQuizSubmissionAnswerRepository(f.g),
		service.WithQuestionGroupRepo(postgres.NewQuizQuestionGroupRepository(f.g)),
		service.WithQuizLifecycle(deps),
	)
}

const mcAnswers = `[{"id":"a","text":"Mitochondria","weight":100},{"id":"b","text":"Nucleus","weight":0}]`

// quiz creates a published graded quiz with nMC one-point multiple-choice
// questions (correct answer "a") and links its assignment.
func (f *lifecycleFixture) quiz(t *testing.T, mutate func(q *models.Quiz), nMC int) (*models.Quiz, []models.QuizQuestion) {
	t.Helper()
	pts := float64(nMC)
	q := &models.Quiz{CourseID: f.courseID, Title: "Cells", QuizType: "assignment", AllowedAttempts: -1,
		Published: true, WorkflowState: "published", PointsPossible: &pts, ScoringPolicy: "keep_highest", ShowCorrectAnswers: true}
	if mutate != nil {
		mutate(q)
	}
	require.NoError(t, f.g.Create(q).Error)
	var qs []models.QuizQuestion
	for i := 0; i < nMC; i++ {
		one := 1.0
		qq := models.QuizQuestion{QuizID: q.ID, Position: i + 1, QuestionType: "multiple_choice",
			QuestionText: fmt.Sprintf("Q%d", i+1), PointsPossible: &one, Answers: mcAnswers, WorkflowState: "active"}
		require.NoError(t, f.svc.CreateQuestion(context.Background(), &qq))
		qs = append(qs, qq)
	}
	_, err := f.svc.SyncQuizAssignment(context.Background(), q, nil)
	require.NoError(t, err)
	return q, qs
}

func (f *lifecycleFixture) take(t *testing.T, quiz *models.Quiz, userID uint, answers map[uint]string) *models.QuizSubmission {
	t.Helper()
	ctx := context.Background()
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, userID, nil)
	require.NoError(t, err)
	for qid, a := range answers {
		_, err := f.svc.AnswerQuestion(ctx, sub.ID, qid, a)
		require.NoError(t, err)
	}
	done, err := f.svc.CompleteSubmission(ctx, sub.ID, userID)
	require.NoError(t, err)
	return done
}

func (f *lifecycleFixture) gradebook(t *testing.T, quiz *models.Quiz, userID uint) *models.Submission {
	t.Helper()
	var reloaded models.Quiz
	require.NoError(t, f.g.First(&reloaded, quiz.ID).Error)
	require.NotNil(t, reloaded.AssignmentID, "graded quiz must be linked to an assignment")
	var sub models.Submission
	err := f.g.Where("assignment_id = ? AND user_id = ?", *reloaded.AssignmentID, userID).First(&sub).Error
	if err != nil {
		return nil
	}
	return &sub
}

// G041: the quiz gets an online_quiz assignment in the course's first
// group; a finished attempt's score lands on it with the late policy.
func TestQuizLifecycle_GradedQuizScoresReachGradebook(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	due := time.Now().Add(-36 * time.Hour) // two days late (day intervals)
	quiz, qs := f.quiz(t, func(q *models.Quiz) { q.DueAt = &due }, 2)

	var a models.Assignment
	require.NoError(t, f.g.First(&a, *quiz.AssignmentID).Error)
	assert.Equal(t, "online_quiz", a.SubmissionTypes)
	assert.Equal(t, "Cells", a.Name)
	assert.Equal(t, models.AssignmentPublished, a.WorkflowState)
	require.NotNil(t, a.AssignmentGroupID)
	assert.Equal(t, f.groupID, *a.AssignmentGroupID)
	require.NotNil(t, a.PointsPossible)
	assert.Equal(t, 2.0, *a.PointsPossible)

	require.NoError(t, f.g.Create(&models.LatePolicy{CourseID: f.courseID, LateSubmissionDeductionEnabled: true,
		LateSubmissionDeduction: 10, LateSubmissionInterval: "day"}).Error)

	f.take(t, quiz, f.students[0], map[uint]string{qs[0].ID: "a", qs[1].ID: "a"})

	sub := f.gradebook(t, quiz, f.students[0])
	require.NotNil(t, sub, "quiz score must create the assignment submission")
	require.NotNil(t, sub.EnteredScore)
	assert.InDelta(t, 2.0, *sub.EnteredScore, 1e-9)
	require.NotNil(t, sub.Score)
	assert.InDelta(t, 1.6, *sub.Score, 1e-9, "two days late at 10%/day of 2 points")
	assert.True(t, sub.Late)
	require.NotNil(t, sub.SubmissionType)
	assert.Equal(t, "online_quiz", *sub.SubmissionType)
	assert.Equal(t, models.SubmissionGraded, sub.WorkflowState)
	assert.NotNil(t, sub.PostedAt, "automatic post policy posts the quiz grade")

	// Switching the quiz to a practice quiz removes it from the gradebook.
	quiz.QuizType = models.QuizTypePractice
	require.NoError(t, f.g.Save(quiz).Error)
	_, err := f.svc.SyncQuizAssignment(ctx, quiz, nil)
	require.NoError(t, err)
	require.NoError(t, f.g.First(&a, a.ID).Error)
	assert.Equal(t, models.AssignmentDeleted, a.WorkflowState)
	assert.Nil(t, quiz.AssignmentID)
}

// G041: a manual post policy keeps the quiz score hidden from the student
// (and the attempt results with it).
func TestQuizLifecycle_ManualPostPolicyHidesResults(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	quiz, qs := f.quiz(t, nil, 1)
	require.NoError(t, f.g.Model(&models.Assignment{}).Where("id = ?", *quiz.AssignmentID).Update("post_policy", "manual").Error)

	done := f.take(t, quiz, f.students[0], map[uint]string{qs[0].ID: "a"})
	sub := f.gradebook(t, quiz, f.students[0])
	require.NotNil(t, sub)
	assert.Nil(t, sub.PostedAt)

	view := f.svc.StudentResultsView(ctx, quiz, done, time.Now())
	assert.False(t, view.Score)
	assert.False(t, view.Results)

	now := time.Now()
	require.NoError(t, f.g.Model(sub).Update("posted_at", now).Error)
	view = f.svc.StudentResultsView(ctx, quiz, done, time.Now())
	assert.True(t, view.Score)
	assert.True(t, view.Results)
	assert.True(t, view.AnswerKey)
}

// G044: the kept score follows the scoring policy across attempts.
func TestQuizLifecycle_ScoringPolicyAcrossAttempts(t *testing.T) {
	for _, tc := range []struct {
		policy string
		want   float64
	}{
		{models.QuizScoringKeepHighest, 2},
		{models.QuizScoringKeepLatest, 1},
		{models.QuizScoringKeepAverage, 1.5},
	} {
		t.Run(tc.policy, func(t *testing.T) {
			f := newLifecycleFixture(t, 1)
			quiz, qs := f.quiz(t, func(q *models.Quiz) { q.ScoringPolicy = tc.policy }, 2)
			f.take(t, quiz, f.students[0], map[uint]string{qs[0].ID: "a", qs[1].ID: "a"})
			second := f.take(t, quiz, f.students[0], map[uint]string{qs[0].ID: "a", qs[1].ID: "b"})
			assert.Equal(t, 2, second.Attempt)

			var latest models.QuizSubmission
			require.NoError(t, f.g.First(&latest, second.ID).Error)
			require.NotNil(t, latest.KeptScore)
			assert.InDelta(t, tc.want, *latest.KeptScore, 1e-9)

			sub := f.gradebook(t, quiz, f.students[0])
			require.NotNil(t, sub)
			assert.InDelta(t, tc.want, *sub.Score, 1e-9)
			assert.Equal(t, 2, sub.Attempt)
		})
	}
}

// G042: an essay keeps the attempt in review until a grader scores it;
// scores, comments and fudge points then complete it and reach the
// gradebook.
func TestQuizLifecycle_ManualGrading(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	quiz, qs := f.quiz(t, func(q *models.Quiz) { five := 5.0; q.PointsPossible = &five }, 1)
	four := 4.0
	essay := models.QuizQuestion{QuizID: quiz.ID, Position: 2, QuestionType: "essay", QuestionText: "Explain osmosis",
		PointsPossible: &four, Answers: "[]", WorkflowState: "active"}
	require.NoError(t, f.svc.CreateQuestion(ctx, &essay))

	done := f.take(t, quiz, f.students[0], map[uint]string{qs[0].ID: "a", essay.ID: "Water moves across a membrane."})
	assert.Equal(t, models.QuizSubmissionPendingReview, done.WorkflowState)
	sub := f.gradebook(t, quiz, f.students[0])
	require.NotNil(t, sub)
	assert.Equal(t, models.SubmissionPendingReview, sub.WorkflowState)
	assert.InDelta(t, 1.0, *sub.Score, 1e-9)

	score := 3.0
	comment := "<b>Good</b> start"
	fudge := 0.5
	updated, answers, err := f.svc.UpdateQuizScores(ctx, quiz, done, service.QuizScoreUpdate{
		FudgePoints: &fudge,
		Questions:   map[uint]service.QuestionScoreUpdate{essay.ID: {Score: &score, Comment: &comment}},
	}, f.teacher)
	require.NoError(t, err)
	assert.Equal(t, models.QuizSubmissionComplete, updated.WorkflowState)
	assert.InDelta(t, 4.5, *updated.Score, 1e-9)
	var essayAns *models.QuizSubmissionAnswer
	for i := range answers {
		if answers[i].QuestionID == essay.ID {
			essayAns = &answers[i]
		}
	}
	require.NotNil(t, essayAns)
	assert.Equal(t, "Good start", essayAns.Comment, "comments are plain text")
	require.NotNil(t, essayAns.GradedVia)
	assert.Equal(t, "manual", *essayAns.GradedVia)

	sub = f.gradebook(t, quiz, f.students[0])
	assert.Equal(t, models.SubmissionGraded, sub.WorkflowState)
	assert.InDelta(t, 4.5, *sub.Score, 1e-9)
	require.NotNil(t, sub.GraderID)
	assert.Equal(t, f.teacher, *sub.GraderID)

	// A question outside the attempt is refused.
	_, _, err = f.svc.UpdateQuizScores(ctx, quiz, updated, service.QuizScoreUpdate{
		Questions: map[uint]service.QuestionScoreUpdate{999999: {Score: &score}},
	}, f.teacher)
	assert.ErrorIs(t, err, service.ErrQuestionNotInSubmission)

	// An in-progress attempt cannot be graded.
	inProgress, err := f.svc.StartSubmission(ctx, quiz.ID, f.students[0], nil)
	require.NoError(t, err)
	_, _, err = f.svc.UpdateQuizScores(ctx, quiz, inProgress, service.QuizScoreUpdate{FudgePoints: &fudge}, f.teacher)
	assert.ErrorIs(t, err, service.ErrQuizAttemptNotFinished)
}

// G047: extra attempts, extra time, manual unlock and extending an
// attempt in progress.
func TestQuizLifecycle_Extensions(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	limit := 10
	quiz, qs := f.quiz(t, func(q *models.Quiz) { q.AllowedAttempts = 1; q.TimeLimit = &limit }, 1)
	student := f.students[0]

	f.take(t, quiz, student, map[uint]string{qs[0].ID: "a"})
	_, err := f.svc.StartSubmission(ctx, quiz.ID, student, nil)
	require.Error(t, err, "second attempt refused without an extension")

	extra, extraTime := 1, 5
	_, _, err = f.svc.SetQuizExtension(ctx, quiz, service.QuizExtensionInput{UserID: student, ExtraAttempts: &extra, ExtraTime: &extraTime}, f.teacher)
	require.NoError(t, err)
	before := time.Now()
	second, err := f.svc.StartSubmission(ctx, quiz.ID, student, nil)
	require.NoError(t, err)
	require.NotNil(t, second.EndAt)
	assert.WithinDuration(t, before.Add(15*time.Minute), *second.EndAt, 5*time.Second, "time limit + extra time")

	// Extend the running attempt by 20 minutes from now.
	twenty := 20
	_, extended, err := f.svc.SetQuizExtension(ctx, quiz, service.QuizExtensionInput{UserID: student, ExtendFromNow: &twenty}, f.teacher)
	require.NoError(t, err)
	require.NotNil(t, extended)
	assert.WithinDuration(t, time.Now().Add(20*time.Minute), *extended.EndAt, 5*time.Second)

	status, err := f.svc.AttemptStatusFor(ctx, quiz, student, false)
	require.NoError(t, err)
	assert.Equal(t, 2, status.AllowedAttempts)
	assert.Equal(t, 1, status.AttemptsUsed)

	// Out-of-range values are refused.
	bad := -1
	_, _, err = f.svc.SetQuizExtension(ctx, quiz, service.QuizExtensionInput{UserID: student, ExtraAttempts: &bad}, f.teacher)
	assert.ErrorIs(t, err, service.ErrInvalidQuizExtension)

	// Roster.
	rows, err := f.svc.QuizModerationRoster(ctx, quiz, 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].ExtraAttempts)
	assert.Equal(t, 5, rows[0].ExtraTime)
	assert.Equal(t, second.ID, rows[0].InProgressID)
}

// G047: "extend the attempt" on an attempt with no deadline leaves it
// without one (it must not start an auto-submit clock).
func TestQuizLifecycle_ExtendUntimedAttemptKeepsNoDeadline(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	quiz, _ := f.quiz(t, nil, 1)
	student := f.students[0]
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, student, nil)
	require.NoError(t, err)
	require.Nil(t, sub.EndAt)

	ten := 10
	_, extended, err := f.svc.SetQuizExtension(ctx, quiz, service.QuizExtensionInput{UserID: student, ExtendFromNow: &ten}, f.teacher)
	require.NoError(t, err)
	assert.Nil(t, extended)
	var reloaded models.QuizSubmission
	require.NoError(t, f.g.First(&reloaded, sub.ID).Error)
	assert.Nil(t, reloaded.EndAt)
}

func TestQuizLifecycle_ManualUnlockAndLockCap(t *testing.T) {
	f := newLifecycleFixture(t, 2)
	ctx := context.Background()
	locked := time.Now().Add(-time.Hour)
	quiz, _ := f.quiz(t, func(q *models.Quiz) { q.LockAt = &locked }, 1)

	_, err := f.svc.StartSubmission(ctx, quiz.ID, f.students[0], nil)
	require.Error(t, err, "locked quiz")
	yes := true
	_, _, err = f.svc.SetQuizExtension(ctx, quiz, service.QuizExtensionInput{UserID: f.students[0], ManuallyUnlocked: &yes}, f.teacher)
	require.NoError(t, err)
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.students[0], nil)
	require.NoError(t, err, "manually unlocked")
	assert.Nil(t, sub.EndAt, "a manual unlock is not capped by lock_at")

	// G048: a timed attempt never runs past lock_at.
	soon := time.Now().Add(10 * time.Minute)
	limit := 60
	quiz2, _ := f.quiz(t, func(q *models.Quiz) { q.LockAt = &soon; q.TimeLimit = &limit }, 1)
	sub2, err := f.svc.StartSubmission(ctx, quiz2.ID, f.students[1], nil)
	require.NoError(t, err)
	require.NotNil(t, sub2.EndAt)
	assert.WithinDuration(t, soon, *sub2.EndAt, time.Second)
}

// G046: a student override on the linked assignment moves that student's
// lock date.
func TestQuizLifecycle_StudentOverrideDates(t *testing.T) {
	f := newLifecycleFixture(t, 2)
	ctx := context.Background()
	locked := time.Now().Add(-time.Hour)
	quiz, _ := f.quiz(t, func(q *models.Quiz) { q.LockAt = &locked }, 1)

	later := time.Now().Add(48 * time.Hour)
	ov := &models.AssignmentOverride{AssignmentID: *quiz.AssignmentID, Title: "Extension", LockAt: &later, DueAt: &later, WorkflowState: "active"}
	require.NoError(t, f.g.Create(ov).Error)
	require.NoError(t, f.g.Create(&models.AssignmentOverrideStudent{AssignmentOverrideID: ov.ID, UserID: f.students[0], AssignmentID: *quiz.AssignmentID}).Error)

	_, err := f.svc.StartSubmission(ctx, quiz.ID, f.students[0], nil)
	require.NoError(t, err, "overridden student may take the quiz")
	_, err = f.svc.StartSubmission(ctx, quiz.ID, f.students[1], nil)
	require.Error(t, err, "everyone else is locked out")

	due, _, lock := f.svc.EffectiveQuizDates(ctx, quiz, f.students[0])
	require.NotNil(t, due)
	require.NotNil(t, lock)
	assert.WithinDuration(t, later, *lock, time.Second)
}

// G048: an attempt left open past its deadline is graded and turned in by
// the server, finished at its deadline.
func TestQuizLifecycle_AutoSubmitOverdue(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	limit := 5
	quiz, qs := f.quiz(t, func(q *models.Quiz) { q.TimeLimit = &limit }, 2)

	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.students[0], nil)
	require.NoError(t, err)
	_, err = f.svc.AnswerQuestion(ctx, sub.ID, qs[0].ID, "a")
	require.NoError(t, err)

	// Not overdue yet: nothing happens.
	n, err := f.svc.AutoSubmitOverdueAttempts(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	ended := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	require.NoError(t, f.g.Model(&models.QuizSubmission{}).Where("id = ?", sub.ID).Update("end_at", ended).Error)
	n, err = f.svc.AutoSubmitOverdueAttempts(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	var got models.QuizSubmission
	require.NoError(t, f.g.First(&got, sub.ID).Error)
	assert.Equal(t, models.QuizSubmissionComplete, got.WorkflowState)
	require.NotNil(t, got.FinishedAt)
	assert.WithinDuration(t, ended, *got.FinishedAt, time.Second)
	assert.InDelta(t, 1.0, *got.Score, 1e-9)

	gb := f.gradebook(t, quiz, f.students[0])
	require.NotNil(t, gb)
	assert.InDelta(t, 1.0, *gb.Score, 1e-9)

	events, err := f.svc.ListQuizEvents(ctx, sub.ID)
	require.NoError(t, err)
	types := []string{}
	for _, e := range events {
		types = append(types, e.EventType)
	}
	assert.Contains(t, types, models.QuizEventSessionStarted)
	assert.Contains(t, types, models.QuizEventQuestionAnswered)
	assert.Contains(t, types, models.QuizEventAutoSubmitted)

	// Already submitted: a second run is a no-op.
	n, err = f.svc.AutoSubmitOverdueAttempts(ctx, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// G053: regrade options after an answer-key change.
func TestQuizLifecycle_Regrade(t *testing.T) {
	f := newLifecycleFixture(t, 2)
	ctx := context.Background()
	quiz, qs := f.quiz(t, nil, 1)
	f.take(t, quiz, f.students[0], map[uint]string{qs[0].ID: "a"}) // right under the old key
	f.take(t, quiz, f.students[1], map[uint]string{qs[0].ID: "b"}) // right under the new key

	// The instructor fixes the key: "b" is correct.
	q := qs[0]
	q.Answers = `[{"id":"a","text":"Mitochondria","weight":0},{"id":"b","text":"Nucleus","weight":100}]`
	require.NoError(t, f.svc.UpdateQuestion(ctx, &q))

	rec, err := f.svc.RegradeQuizQuestion(ctx, quiz, &q, models.QuizRegradeAwardBoth, f.teacher)
	require.NoError(t, err)
	assert.Equal(t, 2, rec.SubmissionsRegraded)
	assert.Equal(t, 1, rec.SubmissionsChanged)
	assert.InDelta(t, 1.0, *f.gradebook(t, quiz, f.students[0]).Score, 1e-9, "award both keeps the old credit")
	assert.InDelta(t, 1.0, *f.gradebook(t, quiz, f.students[1]).Score, 1e-9, "and grants the new one")

	rec, err = f.svc.RegradeQuizQuestion(ctx, quiz, &q, models.QuizRegradeCurrentCorrectOnly, f.teacher)
	require.NoError(t, err)
	assert.Equal(t, 1, rec.SubmissionsChanged)
	assert.InDelta(t, 0.0, *f.gradebook(t, quiz, f.students[0]).Score, 1e-9)

	var first models.QuizSubmission
	require.NoError(t, f.g.Where("quiz_id = ? AND user_id = ?", quiz.ID, f.students[0]).First(&first).Error)
	require.NotNil(t, first.ScoreBeforeRegrade)
	assert.InDelta(t, 1.0, *first.ScoreBeforeRegrade, 1e-9)

	_, err = f.svc.RegradeQuizQuestion(ctx, quiz, &q, models.QuizRegradeFullCredit, f.teacher)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, *f.gradebook(t, quiz, f.students[0]).Score, 1e-9)

	_, err = f.svc.RegradeQuizQuestion(ctx, quiz, &q, "bogus", f.teacher)
	assert.ErrorIs(t, err, service.ErrInvalidRegradeOption)

	history, err := f.svc.ListQuizRegrades(ctx, quiz.ID)
	require.NoError(t, err)
	assert.Len(t, history, 3)
	assert.Equal(t, q.Answers, history[0].AnswersSnapshot)
}

// G057: client events are accepted only for the attempt in progress and
// only from the allowed vocabulary.
func TestQuizLifecycle_ClientEvents(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	quiz, qs := f.quiz(t, nil, 1)
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.students[0], nil)
	require.NoError(t, err)

	ts := time.Now()
	n, err := f.svc.RecordClientQuizEvents(ctx, sub, []service.ClientQuizEvent{
		{EventType: models.QuizEventPageBlurred, ClientTimestamp: &ts},
		{EventType: models.QuizEventQuestionViewed, EventData: json.RawMessage(fmt.Sprintf(`{"question_id":%d}`, qs[0].ID))},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	_, err = f.svc.RecordClientQuizEvents(ctx, sub, []service.ClientQuizEvent{{EventType: models.QuizEventSubmitted}})
	assert.ErrorIs(t, err, service.ErrInvalidQuizEvent, "server-only event types are refused")
	_, err = f.svc.RecordClientQuizEvents(ctx, sub, []service.ClientQuizEvent{{EventType: models.QuizEventPageFocused, EventData: json.RawMessage(`[1,2]`)}})
	assert.ErrorIs(t, err, service.ErrInvalidQuizEvent, "event data must be an object")

	_, err = f.svc.CompleteSubmission(ctx, sub.ID, f.students[0])
	require.NoError(t, err)
	done, err := f.svc.GetSubmission(ctx, sub.ID)
	require.NoError(t, err)
	_, err = f.svc.RecordClientQuizEvents(ctx, done, []service.ClientQuizEvent{{EventType: models.QuizEventPageFocused}})
	assert.Error(t, err, "finished attempts take no client events")

	events, err := f.svc.ListQuizEvents(ctx, sub.ID)
	require.NoError(t, err)
	clientCount := 0
	for _, e := range events {
		if e.Source == "client" {
			clientCount++
		}
	}
	assert.Equal(t, 2, clientCount)
	assert.Equal(t, models.QuizEventSubmitted, events[len(events)-1].EventType)
}

// G041: a graded quiz without an assignment (imported, copied) gets one
// on its first finished attempt, and existing scores are pushed.
func TestQuizLifecycle_LazyAssignmentForUnlinkedQuiz(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	one := 1.0
	quiz := &models.Quiz{CourseID: f.courseID, Title: "Imported", QuizType: "assignment", AllowedAttempts: -1,
		Published: true, WorkflowState: "published", PointsPossible: &one}
	require.NoError(t, f.g.Create(quiz).Error)
	qq := models.QuizQuestion{QuizID: quiz.ID, QuestionType: "multiple_choice", QuestionText: "Q", PointsPossible: &one, Answers: mcAnswers}
	require.NoError(t, f.svc.CreateQuestion(ctx, &qq))

	f.take(t, quiz, f.students[0], map[uint]string{qq.ID: "a"})
	sub := f.gradebook(t, quiz, f.students[0])
	require.NotNil(t, sub)
	assert.InDelta(t, 1.0, *sub.Score, 1e-9)
}

// G041: the hourly link job gives graded quizzes that predate the
// gradebook link their assignment and pushes existing scores; practice
// quizzes and deleted quizzes are left alone, and the attempt records the
// gradebook submission it fed.
func TestQuizLifecycle_LinkGradedQuizzesBackfill(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	one := 1.0
	mk := func(title, quizType, state string) *models.Quiz {
		q := &models.Quiz{CourseID: f.courseID, Title: title, QuizType: quizType, AllowedAttempts: 1,
			Published: state == "published", WorkflowState: state, PointsPossible: &one}
		require.NoError(t, f.g.Create(q).Error)
		return q
	}
	graded := mk("Old graded quiz", "assignment", "published")
	practice := mk("Practice", "practice_quiz", "published")
	deleted := mk("Deleted", "assignment", "deleted")

	// A finished attempt from before the link existed.
	finished := time.Now().Add(-time.Hour)
	attempt := &models.QuizSubmission{QuizID: graded.ID, UserID: f.students[0], Attempt: 1, Score: &one, KeptScore: &one,
		StartedAt: &finished, FinishedAt: &finished, ValidationToken: "tok-backfill", WorkflowState: "complete"}
	require.NoError(t, f.g.Create(attempt).Error)

	n, err := f.svc.LinkGradedQuizzes(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	sub := f.gradebook(t, graded, f.students[0])
	require.NotNil(t, sub, "the earlier attempt's score reaches the gradebook")
	assert.InDelta(t, 1.0, *sub.Score, 1e-9)
	var reloadedAttempt models.QuizSubmission
	require.NoError(t, f.g.First(&reloadedAttempt, attempt.ID).Error)
	require.NotNil(t, reloadedAttempt.SubmissionID)
	assert.Equal(t, sub.ID, *reloadedAttempt.SubmissionID)

	for _, q := range []*models.Quiz{practice, deleted} {
		var r models.Quiz
		require.NoError(t, f.g.First(&r, q.ID).Error)
		assert.Nil(t, r.AssignmentID, "quiz %q must not get an assignment", q.Title)
	}

	// Idempotent: nothing left to link.
	n, err = f.svc.LinkGradedQuizzes(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

// G041: a course copy brings a quiz over without its link but with its
// online_quiz assignment; linking the copied quiz reuses that assignment
// instead of adding a second gradebook column.
func TestQuizLifecycle_LinkAdoptsCopiedQuizAssignment(t *testing.T) {
	f := newLifecycleFixture(t, 0)
	ctx := context.Background()
	one := 1.0
	orphan := &models.Assignment{CourseID: f.courseID, Name: "Copied quiz", SubmissionTypes: "online_quiz",
		GradingType: "points", PointsPossible: &one, WorkflowState: models.AssignmentUnpublished}
	require.NoError(t, f.g.Create(orphan).Error)
	quiz := &models.Quiz{CourseID: f.courseID, Title: "Copied quiz", QuizType: "assignment", AllowedAttempts: 1,
		WorkflowState: "unpublished", PointsPossible: &one}
	require.NoError(t, f.g.Create(quiz).Error)

	n, err := f.svc.LinkGradedQuizzes(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	var reloaded models.Quiz
	require.NoError(t, f.g.First(&reloaded, quiz.ID).Error)
	require.NotNil(t, reloaded.AssignmentID)
	assert.Equal(t, orphan.ID, *reloaded.AssignmentID)
	var count int64
	require.NoError(t, f.g.Model(&models.Assignment{}).
		Where("course_id = ? AND submission_types = ? AND workflow_state <> ?", f.courseID, "online_quiz", "deleted").
		Count(&count).Error)
	assert.Equal(t, int64(1), count, "one gradebook column per quiz")

	// A second quiz with the same title gets its own assignment: the
	// orphan is now linked.
	twin := &models.Quiz{CourseID: f.courseID, Title: "Copied quiz", QuizType: "assignment", AllowedAttempts: 1,
		WorkflowState: "unpublished", PointsPossible: &one}
	require.NoError(t, f.g.Create(twin).Error)
	a, err := f.svc.SyncQuizAssignment(ctx, twin, nil)
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.NotEqual(t, orphan.ID, a.ID)
}

// G041 review: a quiz whose assignment_id points into another course (a
// link carried over by an import) must never delete or edit that
// assignment — AssignmentRepository.Delete is unscoped. Switching the
// quiz to practice only drops the link; the link job gives a graded one
// its own assignment; deleting it removes only its own.
func TestQuizLifecycle_ForeignAssignmentLinkIsNeverTouched(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	other := &models.Course{AccountID: 1, Name: "Chemistry", CourseCode: "CHEM-1", WorkflowState: "available"}
	require.NoError(t, f.g.Create(other).Error)
	foreign := &models.Assignment{CourseID: other.ID, Name: "Cells", SubmissionTypes: "online_quiz",
		WorkflowState: models.AssignmentPublished, Published: true}
	require.NoError(t, f.g.Create(foreign).Error)
	fid := foreign.ID
	assertForeignUntouched := func() {
		t.Helper()
		var a models.Assignment
		require.NoError(t, f.g.First(&a, fid).Error)
		assert.Equal(t, models.AssignmentPublished, a.WorkflowState, "the other course's assignment must stay live")
		assert.Equal(t, other.ID, a.CourseID)
		assert.Equal(t, "Cells", a.Name)
	}

	one := 1.0
	mk := func(title, quizType string) *models.Quiz {
		q := &models.Quiz{CourseID: f.courseID, Title: title, QuizType: quizType, AllowedAttempts: 1,
			Published: true, WorkflowState: "published", PointsPossible: &one, AssignmentID: &fid}
		require.NoError(t, f.g.Create(q).Error)
		return q
	}
	practice := mk("Warm-up", "practice_quiz")
	graded := mk("Cells", "assignment")

	_, err := f.svc.SyncQuizAssignment(ctx, practice, nil)
	require.NoError(t, err)
	assertForeignUntouched()
	var r models.Quiz
	require.NoError(t, f.g.First(&r, practice.ID).Error)
	assert.Nil(t, r.AssignmentID, "the carried-over link is dropped")

	n, err := f.svc.LinkGradedQuizzes(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "a graded quiz linked outside its course still needs its own assignment")
	var g2 models.Quiz
	require.NoError(t, f.g.First(&g2, graded.ID).Error)
	require.NotNil(t, g2.AssignmentID)
	assert.NotEqual(t, fid, *g2.AssignmentID)
	var own models.Assignment
	require.NoError(t, f.g.First(&own, *g2.AssignmentID).Error)
	assert.Equal(t, f.courseID, own.CourseID)
	assertForeignUntouched()

	g2.WorkflowState = "deleted"
	_, err = f.svc.SyncQuizAssignment(ctx, &g2, nil)
	require.NoError(t, err)
	var ownAfter models.Assignment
	require.NoError(t, f.g.First(&ownAfter, own.ID).Error)
	assert.Equal(t, models.AssignmentDeleted, ownAfter.WorkflowState)
	assertForeignUntouched()
}

// G041 review: a Commons import of a graded quiz must not carry the
// source quiz's assignment_id into the target course.
func TestQuizLifecycle_CommonsImportDropsSourceAssignmentLink(t *testing.T) {
	f := newLifecycleFixture(t, 0)
	ctx := context.Background()
	quiz, _ := f.quiz(t, nil, 1)
	var src models.Quiz
	require.NoError(t, f.g.First(&src, quiz.ID).Error)
	require.NotNil(t, src.AssignmentID)

	dst := &models.Course{AccountID: 1, Name: "Biology 2", CourseCode: "BIO-2", WorkflowState: "available"}
	require.NoError(t, f.g.Create(dst).Error)
	commons := service.NewCommonsService(
		postgres.NewSharedContentRepository(f.g), postgres.NewCourseRepository(f.g), postgres.NewAssignmentRepository(f.g),
		postgres.NewPageRepository(f.g), postgres.NewQuizRepository(f.g), postgres.NewModuleRepository(f.g),
		postgres.NewDiscussionTopicRepository(f.g),
	)
	commons.SetContentDeps(postgres.NewQuizQuestionRepository(f.g), postgres.NewModuleItemRepository(f.g))
	shared, err := commons.Publish(ctx, f.teacher, f.courseID, 1, service.CommonsPublishOptions{ResourceType: "quiz", ResourceID: quiz.ID, Title: "Cells"})
	require.NoError(t, err)
	res, err := commons.Import(ctx, f.teacher, dst.ID, shared.ID, 1)
	require.NoError(t, err)
	require.Len(t, res.CreatedIDs["quizzes"], 1)

	var imported models.Quiz
	require.NoError(t, f.g.First(&imported, res.CreatedIDs["quizzes"][0]).Error)
	assert.Nil(t, imported.AssignmentID, "the import must not link the source course's assignment")
}

// G047 / G043 review: the narrow writes used by moderation and one-time
// results touch only their column, and the deadline only while the
// attempt is in progress — a full-row Save of a stale copy would reopen a
// finished attempt or undo a grader's score.
func TestQuizLifecycle_NarrowAttemptWritesDoNotClobber(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	quiz, _ := f.quiz(t, func(q *models.Quiz) { q.OneTimeResults = true }, 1)
	repo := postgres.NewQuizSubmissionRepository(f.g)

	// Extending an attempt that was turned in meanwhile: no write.
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.students[0], nil)
	require.NoError(t, err)
	_, err = f.svc.CompleteSubmission(ctx, sub.ID, f.students[0])
	require.NoError(t, err)
	won, err := repo.SetEndAtIfInProgress(ctx, sub.ID, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, won)
	var row models.QuizSubmission
	require.NoError(t, f.g.First(&row, sub.ID).Error)
	assert.Equal(t, models.QuizSubmissionComplete, row.WorkflowState)
	assert.Nil(t, row.EndAt)

	// Marking results seen from a stale copy keeps a grader's newer score.
	stale := row
	require.NoError(t, f.g.Model(&models.QuizSubmission{}).Where("id = ?", sub.ID).Update("score", 0.5).Error)
	f.svc.MarkResultsSeen(ctx, quiz, &stale, time.Now())
	require.NoError(t, f.g.First(&row, sub.ID).Error)
	require.NotNil(t, row.Score)
	assert.InDelta(t, 0.5, *row.Score, 1e-9, "the grader's score must survive")
	require.NotNil(t, row.ResultsSeenAt)
	first := *row.ResultsSeenAt

	// Only the first view is stamped.
	f.svc.MarkResultsSeen(ctx, quiz, &models.QuizSubmission{ID: sub.ID, QuizID: quiz.ID, WorkflowState: models.QuizSubmissionComplete}, time.Now().Add(time.Hour))
	require.NoError(t, f.g.First(&row, sub.ID).Error)
	assert.WithinDuration(t, first, *row.ResultsSeenAt, time.Second)
}

// unlinkedWithFailures puts quizzes that can never be linked (their
// course does not exist, so creating the assignment fails) ahead of the
// real unlinked quizzes.
type unlinkedWithFailures struct {
	real  repository.QuizUnlinkedGradedFinder
	bogus []models.Quiz
}

func (u unlinkedWithFailures) ListGradedWithoutAssignment(ctx context.Context, afterID uint, limit int) ([]models.Quiz, error) {
	var out []models.Quiz
	for _, b := range u.bogus {
		if b.ID > afterID && len(out) < limit {
			out = append(out, b)
		}
	}
	if len(out) == limit {
		return out, nil
	}
	rest, err := u.real.ListGradedWithoutAssignment(ctx, afterID, limit-len(out))
	return append(out, rest...), err
}

// G041 review: a full batch of quizzes that keep failing to link must not
// stop the link job from reaching the quizzes after them.
func TestQuizLifecycle_LinkJobSkipsPastQuizzesThatFail(t *testing.T) {
	f := newLifecycleFixture(t, 0)
	ctx := context.Background()
	one := 1.0
	real := &models.Quiz{ID: 5000, CourseID: f.courseID, Title: "Real", QuizType: "assignment", AllowedAttempts: 1,
		Published: true, WorkflowState: "published", PointsPossible: &one}
	require.NoError(t, f.g.Create(real).Error)

	var bogus []models.Quiz
	for i := 1; i <= 100; i++ {
		bogus = append(bogus, models.Quiz{ID: uint(i), CourseID: 999999, Title: fmt.Sprintf("Ghost %d", i),
			QuizType: "assignment", WorkflowState: "published"})
	}
	deps := f.deps
	deps.Unlinked = unlinkedWithFailures{real: postgres.NewQuizRepository(f.g), bogus: bogus}
	svc := f.newService(deps)

	n, err := svc.LinkGradedQuizzes(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	var r models.Quiz
	require.NoError(t, f.g.First(&r, real.ID).Error)
	assert.NotNil(t, r.AssignmentID, "the quiz after a full batch of failures is still linked")
}

type noPlannerNotes struct {
	repository.PlannerNoteRepository
}

func (noPlannerNotes) ListByUserID(context.Context, uint, repository.PaginationParams) (*repository.PaginatedResult[models.PlannerNote], error) {
	return &repository.PaginatedResult[models.PlannerNote]{}, nil
}

type noPlannerOverrides struct {
	repository.PlannerOverrideRepository
}

func (noPlannerOverrides) ListByUserID(context.Context, uint) ([]models.PlannerOverride, error) {
	return nil, nil
}

// G041 review: a teacher who takes (previews) a graded quiz gets no row in
// the gradebook; a student's attempt still does.
func TestQuizLifecycle_StaffPreviewStaysOutOfGradebook(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	require.NoError(t, f.g.Create(&models.Enrollment{UserID: f.teacher, CourseID: f.courseID, Type: "TeacherEnrollment",
		Role: "TeacherEnrollment", WorkflowState: "active"}).Error)
	quiz, qs := f.quiz(t, nil, 1)

	f.take(t, quiz, f.teacher, map[uint]string{qs[0].ID: "a"})
	assert.Nil(t, f.gradebook(t, quiz, f.teacher), "a staff preview must not create a gradebook submission")

	f.take(t, quiz, f.students[0], map[uint]string{qs[0].ID: "a"})
	sub := f.gradebook(t, quiz, f.students[0])
	require.NotNil(t, sub)
	assert.InDelta(t, 1.0, *sub.Score, 1e-9)
}

// G041 review: a graded survey with an essay question earns its full
// points and is complete — never "needs grading" in the gradebook.
func TestQuizLifecycle_GradedSurveyEssayIsNotPendingReview(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	quiz, _ := f.quiz(t, func(q *models.Quiz) {
		q.QuizType = models.QuizTypeGradedSurvey
		five := 5.0
		q.PointsPossible = &five
	}, 0)
	essay := models.QuizQuestion{QuizID: quiz.ID, Position: 1, QuestionType: "essay",
		QuestionText: "How was the unit?", Answers: "[]", WorkflowState: "active"}
	require.NoError(t, f.svc.CreateQuestion(ctx, &essay))

	done := f.take(t, quiz, f.students[0], map[uint]string{essay.ID: "Good"})
	assert.Equal(t, models.QuizSubmissionComplete, done.WorkflowState)
	require.NotNil(t, done.Score)
	assert.InDelta(t, 5.0, *done.Score, 1e-9)

	sub := f.gradebook(t, quiz, f.students[0])
	require.NotNil(t, sub)
	assert.Equal(t, models.SubmissionGraded, sub.WorkflowState)
	assert.InDelta(t, 5.0, *sub.Score, 1e-9)

	// A grader comment on the survey keeps it complete.
	comment := "Thanks"
	updated, _, err := f.svc.UpdateQuizScores(ctx, quiz, done, service.QuizScoreUpdate{
		Questions: map[uint]service.QuestionScoreUpdate{essay.ID: {Comment: &comment}}}, f.teacher)
	require.NoError(t, err)
	assert.Equal(t, models.QuizSubmissionComplete, updated.WorkflowState)
}

// G041 review: now that a graded quiz has an online_quiz assignment, the
// planner must still list it once (as the quiz), not once per side.
func TestQuizLifecycle_PlannerListsGradedQuizOnce(t *testing.T) {
	f := newLifecycleFixture(t, 1)
	ctx := context.Background()
	due := time.Now().Add(48 * time.Hour)
	quiz, _ := f.quiz(t, func(q *models.Quiz) { q.DueAt = &due }, 1)
	var linked models.Quiz
	require.NoError(t, f.g.First(&linked, quiz.ID).Error)
	require.NotNil(t, linked.AssignmentID)

	// Planner notes / overrides have no tables in the migration chain;
	// they are empty here and beside the point.
	planner := service.NewPlannerService(noPlannerNotes{}, noPlannerOverrides{},
		postgres.NewEnrollmentRepository(f.g), postgres.NewAssignmentRepository(f.g), postgres.NewQuizRepository(f.g),
		postgres.NewCalendarEventRepository(f.g))
	items, err := planner.GetPlannerItems(ctx, f.students[0], time.Now(), time.Now().Add(7*24*time.Hour))
	require.NoError(t, err)
	var kinds []string
	for _, it := range items {
		kinds = append(kinds, fmt.Sprintf("%s:%d", it.PlannableType, it.PlannableID))
	}
	assert.Equal(t, []string{fmt.Sprintf("quiz:%d", quiz.ID)}, kinds)
}
