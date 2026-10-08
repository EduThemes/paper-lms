package service

import (
	"context"

	"gorm.io/gorm"
)

// CoursePublishService answers the course publish rules that need the
// database (G077).
type CoursePublishService struct {
	db *gorm.DB
}

func NewCoursePublishService(db *gorm.DB) *CoursePublishService {
	return &CoursePublishService{db: db}
}

// HasGradedStudentWork reports whether any real student of the course has
// a scored or graded submission on a live assignment. Canvas refuses to
// unpublish such a course (Course#unpublishable?). The Student View test
// student does not count, and neither do removed (deleted) enrollments.
func (s *CoursePublishService) HasGradedStudentWork(ctx context.Context, courseID uint) (bool, error) {
	var found bool
	err := s.db.WithContext(ctx).Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM submissions s
			JOIN assignments a ON a.id = s.assignment_id
			WHERE a.course_id = ?
			  AND a.workflow_state <> 'deleted'
			  AND (s.score IS NOT NULL OR s.grade IS NOT NULL)
			  AND s.user_id IN (
				SELECT e.user_id FROM enrollments e
				WHERE e.course_id = ? AND e.type = 'StudentEnrollment' AND e.workflow_state <> 'deleted'
			  )
			  AND s.user_id NOT IN (
				SELECT sv.user_id FROM student_view_students sv WHERE sv.course_id = ?
			  )
		)`, courseID, courseID, courseID).Scan(&found).Error
	return found, err
}
