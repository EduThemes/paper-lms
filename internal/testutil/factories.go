package testutil

import (
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

func NewTestUser() *models.User {
	return &models.User{
		ID:           1,
		Name:         "John Doe",
		SortableName: "Doe, John",
		ShortName:    "John Doe",
		LoginID:      "john@example.com",
		Email:        "john@example.com",
		PasswordHash: "$2a$10$placeholder",
		Role:         "user",
		Locale:       "en",
		TimeZone:     "America/New_York",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func NewTestCourse() *models.Course {
	return &models.Course{
		ID:            1,
		AccountID:     1,
		Name:          "Test Course",
		CourseCode:    "TC101",
		WorkflowState: "available",
		DefaultView:   "modules",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func NewTestAssignment() *models.Assignment {
	points := 100.0
	return &models.Assignment{
		ID:             1,
		CourseID:       1,
		Name:           "Test Assignment",
		PointsPossible: &points,
		GradingType:    "points",
		WorkflowState:  "published",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func NewTestSubmission() *models.Submission {
	subType := "online_text_entry"
	body := "My submission"
	return &models.Submission{
		ID:             1,
		AssignmentID:   1,
		UserID:         1,
		SubmissionType: &subType,
		Body:           &body,
		Attempt:        1,
		WorkflowState:  "submitted",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func NewTestEnrollment() *models.Enrollment {
	return &models.Enrollment{
		ID:            1,
		UserID:        1,
		CourseID:      1,
		Type:          "StudentEnrollment",
		Role:          "StudentEnrollment",
		WorkflowState: "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func NewTestQuizQuestion() *models.QuizQuestion {
	points := 1.0
	return &models.QuizQuestion{
		ID:             1,
		QuizID:         1,
		QuestionType:   "multiple_choice",
		QuestionText:   "What is 2+2?",
		PointsPossible: &points,
		Answers:        `[{"id":"a1","text":"4","weight":100},{"id":"a2","text":"3","weight":0}]`,
		WorkflowState:  "active",
	}
}

func NewTestQuizSubmission() *models.QuizSubmission {
	now := time.Now()
	return &models.QuizSubmission{
		ID:            1,
		QuizID:        1,
		UserID:        1,
		Attempt:       1,
		StartedAt:     &now,
		WorkflowState: "untaken",
		ValidationToken: "testtoken123",
	}
}

func NewTestAccessToken() *models.AccessToken {
	return &models.AccessToken{
		ID:            1,
		UserID:        1,
		Token:         "hashedtoken",
		TokenHint:     "abcd",
		Scopes:        "[]",
		Purpose:       "test token",
		WorkflowState: "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}
