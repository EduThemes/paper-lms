package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ErrNoTestStudent is returned when a course has no test student.
var ErrNoTestStudent = errors.New("no test student")

// testStudentPasswordHash is not a bcrypt hash, so no password ever
// matches it: the test student can only be reached through Student View.
const testStudentPasswordHash = "!student-view"

// StudentViewService manages each course's "Test Student" (G076, Canvas
// Student View): a user in the course's account with an active
// StudentEnrollment, marked fake by a student_view_students row. Course
// staff "enter Student View" by switching their session to it.
type StudentViewService struct {
	db *gorm.DB
}

func NewStudentViewService(db *gorm.DB) *StudentViewService {
	return &StudentViewService{db: db}
}

// Ensure returns the course's test student, creating the user, its
// enrollment and the marker row on first use, and reviving the
// enrollment if it was concluded or deleted since.
func (s *StudentViewService) Ensure(ctx context.Context, course *models.Course) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var marker models.StudentViewStudent
		err := tx.Where("course_id = ?", course.ID).First(&marker).Error
		switch {
		case err == nil:
			if err := tx.First(&user, marker.UserID).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			suffix, rerr := randomHex(6)
			if rerr != nil {
				return rerr
			}
			login := fmt.Sprintf("test_student_%d_%s@student-view.invalid", course.ID, suffix)
			user = models.User{
				AccountID:    course.AccountID,
				Name:         "Test Student",
				SortableName: "Student, Test",
				ShortName:    "Test Student",
				LoginID:      login,
				Email:        login,
				PasswordHash: testStudentPasswordHash,
				Role:         "user",
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			marker = models.StudentViewStudent{CourseID: course.ID, UserID: user.ID}
			if err := tx.Create(&marker).Error; err != nil {
				return err
			}
		default:
			return err
		}

		var enrollment models.Enrollment
		err = tx.Where("user_id = ? AND course_id = ? AND type = ?", user.ID, course.ID, "StudentEnrollment").
			Order("id DESC").First(&enrollment).Error
		switch {
		case err == nil:
			if enrollment.WorkflowState != models.EnrollmentActive {
				return tx.Model(&enrollment).Update("workflow_state", models.EnrollmentActive).Error
			}
			return nil
		case errors.Is(err, gorm.ErrRecordNotFound):
			return tx.Create(&models.Enrollment{
				UserID:            user.ID,
				CourseID:          course.ID,
				Type:              "StudentEnrollment",
				Role:              "StudentEnrollment",
				WorkflowState:     models.EnrollmentActive,
				PseudonymPoolCode: "animals_v1",
			}).Error
		default:
			return err
		}
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// TestStudentCourse returns the course whose test student userID is.
func (s *StudentViewService) TestStudentCourse(ctx context.Context, userID uint) (uint, error) {
	var marker models.StudentViewStudent
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&marker).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrNoTestStudent
		}
		return 0, err
	}
	return marker.CourseID, nil
}

// Reset retires the course's test student (Canvas "Reset Student"): its
// enrollment is deleted, so its submissions and progress leave the
// course, and the marker goes, so the next entry starts a fresh one.
func (s *StudentViewService) Reset(ctx context.Context, courseID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var marker models.StudentViewStudent
		if err := tx.Where("course_id = ?", courseID).First(&marker).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if err := tx.Model(&models.Enrollment{}).
			Where("user_id = ? AND course_id = ?", marker.UserID, courseID).
			Update("workflow_state", models.EnrollmentDeleted).Error; err != nil {
			return err
		}
		return tx.Delete(&marker).Error
	})
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
