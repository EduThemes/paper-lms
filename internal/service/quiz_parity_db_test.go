package service_test

// Real-Postgres integration tests for the quizzes-lane parity fixes:
//   - B030: bank-linked question groups materialize the drawn bank entries
//     into quiz_questions (never mixing bank-entry ids into quiz-question
//     ids) and refuse a bank from another course.
//   - B134: CompleteSubmission finalizes with a conditional UPDATE, so a
//     second concurrent /complete loses and completion callbacks fire once.
//   - B132: expired / not-yet-effective accommodations do not extend quiz
//     time limits.
//   - B034 / B128 repository reads (own-attempt listing, published-only list).
//
// Skipped when PARITY_DB_URL / DATABASE_URL is unset (freshDB).

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"gorm.io/gorm"
)

type quizDBFixture struct {
	g        *gorm.DB
	svc      *service.QuizService
	courseID uint
	other    uint // a course in another tenant
	userID   uint
}

func newQuizDBFixture(t *testing.T, opts ...func(*service.QuizService)) *quizDBFixture {
	t.Helper()
	g, cleanup := freshDB(t)
	t.Cleanup(cleanup)

	for _, id := range []uint{1, 2} {
		if err := g.Exec(
			`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
			 VALUES (?, ?, 'active', 'off', 'en', 'higher_ed', 500)
			 ON CONFLICT (id) DO NOTHING`,
			id, fmt.Sprintf("Account %d", id),
		).Error; err != nil {
			t.Fatalf("seed account %d: %v", id, err)
		}
	}
	c1 := &models.Course{AccountID: 1, Name: "Course A", CourseCode: "A-1", WorkflowState: "available"}
	c2 := &models.Course{AccountID: 2, Name: "Course B", CourseCode: "B-1", WorkflowState: "available"}
	for _, c := range []*models.Course{c1, c2} {
		if err := g.Create(c).Error; err != nil {
			t.Fatalf("seed course: %v", err)
		}
	}
	u := &models.User{AccountID: 1, Name: "Student One", LoginID: "student.one", Email: "student.one@example.com", Role: "user"}
	if err := g.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	base := []func(*service.QuizService){
		service.WithQuestionGroupRepo(postgres.NewQuizQuestionGroupRepository(g)),
		service.WithBankEntryRepo(postgres.NewQuestionBankEntryRepository(g)),
		service.WithQuestionBankRepo(postgres.NewQuestionBankRepository(g)),
	}
	svc := service.NewQuizService(
		postgres.NewQuizRepository(g),
		postgres.NewQuizQuestionRepository(g),
		postgres.NewQuizSubmissionRepository(g),
		postgres.NewQuizSubmissionAnswerRepository(g),
		append(base, opts...)...,
	)
	return &quizDBFixture{g: g, svc: svc, courseID: c1.ID, other: c2.ID, userID: u.ID}
}

func (f *quizDBFixture) quiz(t *testing.T, published bool) *models.Quiz {
	t.Helper()
	state := "unpublished"
	if published {
		state = "published"
	}
	q := &models.Quiz{CourseID: f.courseID, Title: "Quiz", QuizType: "assignment", AllowedAttempts: -1, Published: published, WorkflowState: state}
	if err := f.g.Create(q).Error; err != nil {
		t.Fatalf("seed quiz: %v", err)
	}
	return q
}

func (f *quizDBFixture) bank(t *testing.T, courseID uint, entries int) *models.QuestionBank {
	t.Helper()
	b := &models.QuestionBank{CourseID: courseID, Title: "Bank", WorkflowState: "active"}
	if err := f.g.Create(b).Error; err != nil {
		t.Fatalf("seed bank: %v", err)
	}
	for i := 0; i < entries; i++ {
		e := &models.QuestionBankEntry{
			QuestionBankID: b.ID,
			QuestionType:   "multiple_choice",
			QuestionText:   fmt.Sprintf("Bank question %d", i+1),
			PointsPossible: 2,
			Answers:        `[{"id":"a","text":"yes","weight":100},{"id":"b","text":"no","weight":0}]`,
		}
		if err := f.g.Create(e).Error; err != nil {
			t.Fatalf("seed entry: %v", err)
		}
	}
	return b
}

// B030 — drawn bank entries become quiz questions of THIS quiz.
func TestQuizDB_BankGroupMaterializesEntries(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	quiz := f.quiz(t, true)
	bank := f.bank(t, f.courseID, 3)

	// A decoy quiz_questions row whose id collides with a bank entry id in
	// another quiz: pre-fix, the entry id was resolved against this table.
	decoyQuiz := &models.Quiz{CourseID: f.other, Title: "Other tenant", QuizType: "assignment", WorkflowState: "published", Published: true}
	if err := f.g.Create(decoyQuiz).Error; err != nil {
		t.Fatal(err)
	}

	bankID := bank.ID
	group := &models.QuizQuestionGroup{QuizID: quiz.ID, Name: "Pool", PickCount: 2, QuestionBankID: &bankID}
	if err := f.svc.CreateQuestionGroup(ctx, group); err != nil {
		t.Fatal(err)
	}

	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.userID, nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	var ids []uint
	if err := json.Unmarshal([]byte(sub.SelectedQuestions), &ids); err != nil {
		t.Fatalf("selected_questions: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("want 2 drawn questions, got %v", ids)
	}
	qs, err := f.svc.GetSubmissionQuestions(ctx, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("want 2 resolved questions, got %d", len(qs))
	}
	for _, q := range qs {
		if q.QuizID != quiz.ID {
			t.Errorf("served question %d from quiz %d, want quiz %d", q.ID, q.QuizID, quiz.ID)
		}
		if q.QuestionBankEntryID == nil || q.QuizQuestionGroupID == nil || *q.QuizQuestionGroupID != group.ID {
			t.Errorf("question %d not a materialized bank copy: %+v", q.ID, q)
		}
	}

	// Answering + completing grades the drawn questions at the group's
	// points per question — 1 when the group sets none (Canvas
	// question_points default), never the bank entry's own 2 points — so a
	// perfect attempt matches the quiz total the editor saves (pick 2 x 1).
	for _, id := range ids {
		if _, err := f.svc.AnswerQuestion(ctx, sub.ID, id, "a"); err != nil {
			t.Fatalf("answer %d: %v", id, err)
		}
	}
	done, err := f.svc.CompleteSubmission(ctx, sub.ID, f.userID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Score == nil || *done.Score != 2 {
		t.Errorf("score = %v, want 2", done.Score)
	}

	// A second attempt reuses the same materialized copies (no growth
	// beyond the bank size).
	sub2, err := f.svc.StartSubmission(ctx, quiz.ID, f.userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = sub2
	var copies int64
	f.g.Model(&models.QuizQuestion{}).Where("quiz_id = ? AND question_bank_entry_id IS NOT NULL", quiz.ID).Count(&copies)
	if copies > 3 {
		t.Errorf("materialized %d copies for a 3-entry bank", copies)
	}
}

// B030 — a group may not draw from another course's (tenant's) bank.
func TestQuizDB_BankGroupRejectsForeignBank(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	quiz := f.quiz(t, true)
	foreign := f.bank(t, f.other, 2)

	if err := f.svc.ValidateQuestionBankForCourse(ctx, foreign.ID, quiz.CourseID); err == nil {
		t.Fatal("foreign bank accepted for this course")
	}

	// Even if a legacy row already links one, nothing is drawn from it.
	bid := foreign.ID
	if err := f.g.Create(&models.QuizQuestionGroup{QuizID: quiz.ID, Name: "Bad", PickCount: 2, QuestionBankID: &bid}).Error; err != nil {
		t.Fatal(err)
	}
	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sub.SelectedQuestions != "" && sub.SelectedQuestions != "null" && sub.SelectedQuestions != "[]" {
		t.Errorf("drew questions from a foreign bank: %s", sub.SelectedQuestions)
	}
}

// B134 — only one concurrent completion wins; callbacks fire once.
func TestQuizDB_DoubleCompleteFiresOnce(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	quiz := f.quiz(t, true)
	pts := 1.0
	q := &models.QuizQuestion{QuizID: quiz.ID, QuestionType: "multiple_choice", QuestionText: "Q", PointsPossible: &pts,
		Answers: `[{"id":"a","text":"yes","weight":100}]`, WorkflowState: "active"}
	if err := f.g.Create(q).Error; err != nil {
		t.Fatal(err)
	}

	var fired int32
	f.svc.OnCompleted(func(context.Context, uint) { atomic.AddInt32(&fired, 1) })

	sub, err := f.svc.StartSubmission(ctx, quiz.ID, f.userID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AnswerQuestion(ctx, sub.ID, q.ID, "a"); err != nil {
		t.Fatal(err)
	}

	// Fire several /complete calls at once (double click + timer). However
	// they interleave, exactly one may succeed and fire callbacks.
	const n = 6
	var wins int32
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for i := 0; i < n; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			if _, err := f.svc.CompleteSubmission(ctx, sub.ID, f.userID); err == nil {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	start.Done()
	done.Wait()

	if got := atomic.LoadInt32(&wins); got != 1 {
		t.Fatalf("%d concurrent completions succeeded, want exactly 1", got)
	}
	time.Sleep(150 * time.Millisecond) // callbacks run in goroutines
	if got := atomic.LoadInt32(&fired); got != 1 {
		t.Errorf("completion callbacks fired %d times, want 1", got)
	}
}

// B134 — the repository finalize is conditional on 'untaken'.
func TestQuizDB_FinalizeIfInProgressIsConditional(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	quiz := f.quiz(t, true)
	repo := postgres.NewQuizSubmissionRepository(f.g)
	now := time.Now()
	sub := &models.QuizSubmission{QuizID: quiz.ID, UserID: f.userID, Attempt: 1, StartedAt: &now, ValidationToken: "tok", WorkflowState: "untaken"}
	if err := repo.Create(ctx, sub); err != nil {
		t.Fatal(err)
	}
	score := 1.0
	sub.Score, sub.KeptScore, sub.FinishedAt, sub.WorkflowState = &score, &score, &now, "complete"
	won, err := repo.FinalizeIfInProgress(ctx, sub)
	if err != nil || !won {
		t.Fatalf("first finalize: won=%v err=%v", won, err)
	}
	won, err = repo.FinalizeIfInProgress(ctx, sub)
	if err != nil || won {
		t.Fatalf("second finalize must lose: won=%v err=%v", won, err)
	}
}

// B034 / B128 repository reads.
func TestQuizDB_StudentScopedListings(t *testing.T) {
	f := newQuizDBFixture(t)
	ctx := context.Background()
	pub := f.quiz(t, true)
	_ = f.quiz(t, false)

	quizRepo := postgres.NewQuizRepository(f.g)
	list, err := quizRepo.ListPublishedByCourseID(ctx, f.courseID, repository.PaginationParams{Page: 1, PerPage: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != pub.ID || list.TotalCount != 1 {
		t.Fatalf("published list = %+v", list.Items)
	}

	other := &models.User{AccountID: 1, Name: "Student Two", LoginID: "student.two", Email: "student.two@example.com", Role: "user"}
	if err := f.g.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	subRepo := postgres.NewQuizSubmissionRepository(f.g)
	now := time.Now()
	for _, uid := range []uint{f.userID, other.ID} {
		s := &models.QuizSubmission{QuizID: pub.ID, UserID: uid, Attempt: 1, StartedAt: &now, ValidationToken: "t", WorkflowState: "complete"}
		if err := subRepo.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	mine, err := f.svc.ListSubmissionsForUsers(ctx, pub.ID, []uint{f.userID})
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || mine[0].UserID != f.userID {
		t.Fatalf("own submissions = %+v", mine)
	}
}

// B132 — expired accommodations do not extend the time limit.
func TestQuizDB_ExpiredAccommodationIgnored(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := postgres.NewStudentAccommodationRepository(g)
	courseID := uint(77)
	mult := 1.5
	past := time.Now().AddDate(0, -6, 0)
	ended := time.Now().AddDate(0, -1, 0)
	future := time.Now().AddDate(0, 1, 0)
	rows := []*models.StudentAccommodation{
		{UserID: 5, CourseID: &courseID, AccommodationType: "extended_time", TimeMultiplier: &mult, Status: "active", CreatedByID: 1, EffectiveFrom: past, EffectiveUntil: &ended},
		{UserID: 5, CourseID: &courseID, AccommodationType: "extended_time", TimeMultiplier: &mult, Status: "active", CreatedByID: 1, EffectiveFrom: future},
	}
	for _, r := range rows {
		if err := repo.Create(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	svc := service.NewAccommodationService(repo, postgres.NewAccommodationApplicationRepository(g))
	limit := 30
	adj, err := svc.ApplyAccommodationsToQuiz(ctx, 5, &courseID, &limit)
	if err != nil {
		t.Fatal(err)
	}
	if adj != nil {
		t.Fatalf("expired/future accommodation applied: %+v", adj)
	}

	current := &models.StudentAccommodation{UserID: 5, CourseID: &courseID, AccommodationType: "extended_time", TimeMultiplier: &mult, Status: "active", CreatedByID: 1, EffectiveFrom: past}
	if err := repo.Create(ctx, current); err != nil {
		t.Fatal(err)
	}
	adj, err = svc.ApplyAccommodationsToQuiz(ctx, 5, &courseID, &limit)
	if err != nil || adj == nil || adj.AdjustedTimeLimit != 45 {
		t.Fatalf("current accommodation not applied: %+v err=%v", adj, err)
	}
}
