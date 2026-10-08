package service_test

// Unit tests for the quizzes-lane parity fixes that do not need Postgres:
// B025 (question membership), B033 (staff can read answers), B129 (group
// delete ungroups; dangling group ids are treated as ungrouped) and B023
// (student-safe answer redaction).

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// memGroupRepo is a tiny in-memory QuizQuestionGroupRepository.
type memGroupRepo struct {
	groups  map[uint]*models.QuizQuestionGroup
	deleted []uint
}

func (r *memGroupRepo) Create(_ context.Context, g *models.QuizQuestionGroup) error {
	g.ID = uint(len(r.groups) + 1)
	r.groups[g.ID] = g
	return nil
}
func (r *memGroupRepo) FindByID(_ context.Context, id uint) (*models.QuizQuestionGroup, error) {
	if g, ok := r.groups[id]; ok {
		return g, nil
	}
	return nil, errors.New("not found")
}
func (r *memGroupRepo) Update(_ context.Context, g *models.QuizQuestionGroup) error {
	r.groups[g.ID] = g
	return nil
}
func (r *memGroupRepo) Delete(_ context.Context, id uint) error {
	delete(r.groups, id)
	r.deleted = append(r.deleted, id)
	return nil
}
func (r *memGroupRepo) ListByQuizID(_ context.Context, quizID uint) ([]models.QuizQuestionGroup, error) {
	out := []models.QuizQuestionGroup{}
	for _, g := range r.groups {
		if g.QuizID == quizID {
			out = append(out, *g)
		}
	}
	return out, nil
}

func newParitySvc(opts ...func(*service.QuizService)) (*service.QuizService, *mocks.MockQuizQuestionRepository, *mocks.MockQuizSubmissionRepository, *mocks.MockQuizSubmissionAnswerRepository, *mocks.MockQuizRepository) {
	qr := new(mocks.MockQuizQuestionRepository)
	sr := new(mocks.MockQuizSubmissionRepository)
	ar := new(mocks.MockQuizSubmissionAnswerRepository)
	quizRepo := new(mocks.MockQuizRepository)
	return service.NewQuizService(quizRepo, qr, sr, ar, opts...), qr, sr, ar, quizRepo
}

// B025 — answering a question from another quiz is refused.
func TestAnswerQuestion_RejectsQuestionFromOtherQuiz(t *testing.T) {
	svc, qr, sr, ar, _ := newParitySvc()
	ctx := context.Background()
	end := time.Now().Add(time.Hour)
	sr.On("FindByID", ctx, uint(1)).Return(&models.QuizSubmission{ID: 1, QuizID: 1, UserID: 10, WorkflowState: "untaken", EndAt: &end}, nil)
	qr.On("FindByID", ctx, uint(999)).Return(&models.QuizQuestion{ID: 999, QuizID: 2}, nil)

	_, err := svc.AnswerQuestion(ctx, 1, 999, "a1")
	assert.ErrorIs(t, err, service.ErrQuestionNotInSubmission)
	ar.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	ar.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// B025 — a question outside the attempt's personalized set is refused.
func TestAnswerQuestion_RejectsQuestionNotSelectedForAttempt(t *testing.T) {
	svc, qr, sr, ar, _ := newParitySvc()
	ctx := context.Background()
	sr.On("FindByID", ctx, uint(1)).Return(&models.QuizSubmission{ID: 1, QuizID: 1, UserID: 10, WorkflowState: "untaken", SelectedQuestions: "[100,101]"}, nil)
	qr.On("FindByID", ctx, uint(102)).Return(&models.QuizQuestion{ID: 102, QuizID: 1}, nil)

	_, err := svc.AnswerQuestion(ctx, 1, 102, "a1")
	assert.ErrorIs(t, err, service.ErrQuestionNotInSubmission)
	ar.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B025 — stray answers already stored against other quizzes' questions do
// not inflate the score.
func TestCompleteSubmission_IgnoresForeignAnswers(t *testing.T) {
	svc, qr, sr, ar, _ := newParitySvc()
	ctx := context.Background()
	started := time.Now().Add(-time.Minute)
	sub := &models.QuizSubmission{ID: 1, QuizID: 1, UserID: 10, StartedAt: &started, WorkflowState: "untaken"}
	pts := 5.0
	own := &models.QuizQuestion{ID: 100, QuizID: 1, QuestionType: "multiple_choice", PointsPossible: &pts, Answers: `[{"id":"a","weight":100}]`}
	foreign := &models.QuizQuestion{ID: 900, QuizID: 2, QuestionType: "multiple_choice", PointsPossible: &pts, Answers: `[{"id":"a","weight":100}]`}

	sr.On("FindByID", ctx, uint(1)).Return(sub, nil)
	ar.On("ListBySubmissionID", ctx, uint(1)).Return([]models.QuizSubmissionAnswer{
		{ID: 1, QuizSubmissionID: 1, QuestionID: 100, Answer: "a"},
		{ID: 2, QuizSubmissionID: 1, QuestionID: 900, Answer: "a"},
	}, nil)
	qr.On("FindByID", ctx, uint(100)).Return(own, nil)
	qr.On("FindByID", ctx, uint(900)).Return(foreign, nil)
	ar.On("Update", ctx, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Return(nil)
	sr.On("Update", ctx, sub).Return(nil)

	res, err := svc.CompleteSubmission(ctx, 1, 10)
	require.NoError(t, err)
	require.NotNil(t, res.Score)
	assert.Equal(t, 5.0, *res.Score, "foreign question's points must not be added")
}

// Group points_per_item overrides the per-question points (Canvas
// "points per question" on a question group).
func TestCompleteSubmission_AppliesGroupPointsPerItem(t *testing.T) {
	ppi := 3.0
	groups := &memGroupRepo{groups: map[uint]*models.QuizQuestionGroup{7: {ID: 7, QuizID: 1, PickCount: 1, PointsPerItem: &ppi}}}
	svc, qr, sr, ar, _ := newParitySvc(service.WithQuestionGroupRepo(groups))
	ctx := context.Background()
	sub := &models.QuizSubmission{ID: 1, QuizID: 1, UserID: 10, WorkflowState: "untaken"}
	pts := 1.0
	gid := uint(7)
	q := &models.QuizQuestion{ID: 100, QuizID: 1, QuizQuestionGroupID: &gid, QuestionType: "true_false", PointsPossible: &pts, Answers: `[{"id":"true","weight":100}]`}
	sr.On("FindByID", ctx, uint(1)).Return(sub, nil)
	ar.On("ListBySubmissionID", ctx, uint(1)).Return([]models.QuizSubmissionAnswer{{ID: 1, QuestionID: 100, Answer: "true"}}, nil)
	qr.On("FindByID", ctx, uint(100)).Return(q, nil)
	ar.On("Update", ctx, mock.Anything).Return(nil)
	sr.On("Update", ctx, sub).Return(nil)

	res, err := svc.CompleteSubmission(ctx, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, 3.0, *res.Score)
}

// B033 — staff may read a student's answers; the owner may not read an
// in-progress attempt.
func TestListSubmissionAnswers_StaffAccess(t *testing.T) {
	svc, _, sr, ar, _ := newParitySvc()
	ctx := context.Background()
	sr.On("FindByID", ctx, uint(1)).Return(&models.QuizSubmission{ID: 1, UserID: 10, WorkflowState: "complete"}, nil)
	sr.On("FindByID", ctx, uint(2)).Return(&models.QuizSubmission{ID: 2, UserID: 10, WorkflowState: "untaken"}, nil)
	ar.On("ListBySubmissionID", ctx, uint(1)).Return([]models.QuizSubmissionAnswer{{ID: 1}}, nil)
	ar.On("ListBySubmissionID", ctx, uint(2)).Return([]models.QuizSubmissionAnswer{{ID: 2}}, nil)

	got, err := svc.ListSubmissionAnswers(ctx, 1, true)
	require.NoError(t, err)
	assert.Len(t, got, 1)

	_, err = svc.ListSubmissionAnswers(ctx, 2, false)
	assert.Error(t, err)
	got, err = svc.ListSubmissionAnswers(ctx, 2, true)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

// B129 — deleting a group ungroups its questions.
func TestDeleteQuestionGroup_UngroupsQuestions(t *testing.T) {
	groups := &memGroupRepo{groups: map[uint]*models.QuizQuestionGroup{7: {ID: 7, QuizID: 1}}}
	svc, qr, _, _, _ := newParitySvc(service.WithQuestionGroupRepo(groups))
	ctx := context.Background()
	gid := uint(7)
	entry := uint(55)
	qr.On("ListByGroupID", ctx, uint(7)).Return([]models.QuizQuestion{
		{ID: 1, QuizID: 1, QuizQuestionGroupID: &gid},
		{ID: 2, QuizID: 1, QuizQuestionGroupID: &gid, QuestionBankEntryID: &entry},
	}, nil)
	qr.On("Update", ctx, mock.MatchedBy(func(q *models.QuizQuestion) bool {
		return q.ID == 1 && q.QuizQuestionGroupID == nil
	})).Return(nil).Once()

	require.NoError(t, svc.DeleteQuestionGroup(ctx, 7))
	qr.AssertExpectations(t)
	assert.Equal(t, []uint{7}, groups.deleted)
}

// B129 — a question whose group no longer exists is still served.
func TestStartSubmission_DanglingGroupQuestionIsUngrouped(t *testing.T) {
	groups := &memGroupRepo{groups: map[uint]*models.QuizQuestionGroup{8: {ID: 8, QuizID: 1, PickCount: 1}}}
	svc, qr, sr, _, quizRepo := newParitySvc(service.WithQuestionGroupRepo(groups))
	ctx := context.Background()
	dangling := uint(99)
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(&models.Quiz{ID: 1, AllowedAttempts: -1}, nil)
	sr.On("FindByQuizAndUser", ctx, uint(1), uint(10)).Return(nil, errors.New("none"))
	qr.On("ListByQuizID", ctx, uint(1), repository.PaginationParams{Page: 1, PerPage: 10000}).Return(&repository.PaginatedResult[models.QuizQuestion]{
		Items: []models.QuizQuestion{{ID: 1, QuizID: 1}, {ID: 2, QuizID: 1, QuizQuestionGroupID: &dangling}},
	}, nil)
	qr.On("ListByGroupID", ctx, uint(8)).Return([]models.QuizQuestion{}, nil)
	var created *models.QuizSubmission
	sr.On("Create", ctx, mock.AnythingOfType("*models.QuizSubmission")).Run(func(a mock.Arguments) {
		created = a.Get(1).(*models.QuizSubmission)
	}).Return(nil)

	_, err := svc.StartSubmission(ctx, 1, 10, nil)
	require.NoError(t, err)
	var ids []uint
	require.NoError(t, json.Unmarshal([]byte(created.SelectedQuestions), &ids))
	assert.ElementsMatch(t, []uint{1, 2}, ids)
}

// B128 — a soft-deleted quiz cannot be started.
func TestStartSubmission_DeletedQuizRefused(t *testing.T) {
	svc, _, sr, _, quizRepo := newParitySvc()
	ctx := context.Background()
	quizRepo.On("FindByID", ctx, uint(1), uint(0)).Return(&models.Quiz{ID: 1, WorkflowState: "deleted"}, nil)
	_, err := svc.StartSubmission(ctx, 1, 10, nil)
	assert.Error(t, err)
	sr.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// B023 — the student view of answers never carries the key.
func TestStudentSafeQuizAnswers(t *testing.T) {
	cases := []struct {
		qType, answers string
		mustNot        []string
		must           []string
	}{
		{"multiple_choice", `[{"id":"a","text":"4","weight":100,"comments":"right!"},{"id":"b","text":"5","weight":0}]`, []string{"weight", "comments", "right!"}, []string{`"id":"a"`, `"text":"4"`}},
		{"short_answer", `[{"id":"s","text":"Paris","weight":100}]`, []string{"Paris"}, nil},
		{"numerical_question", `[{"id":"n","text":"42","margin":"0.5","weight":100}]`, []string{"42", "margin"}, nil},
		{"fill_in_the_blank", `[{"id":"f","text":"mitochondria","weight":100}]`, []string{"mitochondria"}, nil},
		{"formula", `[{"id":"f","answer_value":"10","tolerance":1,"weight":100}]`, []string{"answer_value", "10"}, nil},
		{"multiple_dropdown", `[{"id":"o1","text":"red","weight":100,"blank_id":"c"},{"id":"o2","text":"blue","weight":0,"blank_id":"c"}]`, []string{"weight"}, []string{`"blank_id":"c"`}},
		{"matching", `[{"left":"France","right_id":"paris","right":"Paris","weight":100},{"left":"UK","right_id":"london","right":"London","weight":100}]`, []string{"right_id", "weight"}, []string{`"left":"France"`, `"Paris"`}},
		{"fill_in_multiple_blanks", `{"color":["red"],"city":["Phoenix"]}`, []string{"red", "Phoenix"}, []string{`"blank_id":"color"`}},
		{"categorization", `[{"id":"c","weight":100,"items":[{"id":"i1","text":"apple"}],"buckets":[{"id":"b1","label":"Fruit","item_ids":["i1"]}]}]`, []string{"item_ids"}, []string{`"apple"`, `"Fruit"`}},
		{"hot_spot", `[{"id":"h","weight":100,"image_url":"/img.png","regions":[{"x":0.1,"y":0.1,"w":0.2,"h":0.2}]}]`, []string{"regions", "0.2"}, []string{"/img.png"}},
		{"essay", `[{"id":"x","text":"model answer"}]`, []string{"model answer"}, nil},
		{"some_future_type", `[{"id":"x","text":"secret","weight":100}]`, []string{"secret"}, nil},
	}
	for _, tc := range cases {
		out := service.StudentSafeQuizAnswers(tc.qType, tc.answers, 42)
		for _, s := range tc.mustNot {
			assert.NotContains(t, out, s, "%s leaked %q: %s", tc.qType, s, out)
		}
		for _, s := range tc.must {
			assert.Contains(t, out, s, "%s missing %q: %s", tc.qType, s, out)
		}
	}

	// Ordering: the presentation must be a UNIFORM shuffle. If the
	// authored (= correct) order were excluded, a two-item question would
	// always come back reversed and the student could read the answer off
	// the page. Across seeds both orders of a 2-item list must occur.
	twoItems := `[{"id":"o1","text":"1"},{"id":"o2","text":"2"}]`
	seenOrders := map[string]bool{}
	for seed := int64(0); seed < 64; seed++ {
		seenOrders[service.StudentSafeQuizAnswers("ordering", twoItems, seed)] = true
	}
	assert.Len(t, seenOrders, 2, "2-item ordering presentation must not be deterministic: %v", seenOrders)

	// Same for a two-pair matching pool: the pool order must not encode
	// the pairing.
	twoPairs := `[{"left":"France","right_id":"paris","right":"Paris","weight":100},{"left":"UK","right_id":"london","right":"London","weight":100}]`
	poolFirst := map[bool]bool{}
	for seed := int64(0); seed < 64; seed++ {
		out := service.StudentSafeQuizAnswers("matching", twoPairs, seed)
		poolFirst[strings.Index(out, `"id":"paris"`) < strings.Index(out, `"id":"london"`)] = true
	}
	assert.Len(t, poolFirst, 2, "2-pair matching pool order must not be deterministic")
}
