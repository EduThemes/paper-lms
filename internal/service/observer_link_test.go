package service_test

// B177 regression: observer pairing is a durable link, not a snapshot of
// the child's enrollments at pairing time.

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestObserverLink_FollowsStudentEnrollments(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()

	enrollmentRepo := postgres.NewEnrollmentRepository(g)
	svc := service.NewObserverService(enrollmentRepo, postgres.NewCourseRepository(g), postgres.NewUserRepository(g)).
		SetObserverLinkStore(postgres.NewObserverLinkRepository(g))

	parent := &models.User{AccountID: 1, Name: "Pat Parent", LoginID: "pat.parent", Email: "pat.parent@example.com"}
	child := &models.User{AccountID: 1, Name: "Casey Child", LoginID: "casey.child", Email: "casey.child@example.com"}
	for _, u := range []*models.User{parent, child} {
		if err := g.Create(u).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	fall := &models.Course{AccountID: 1, Name: "Fall Math", CourseCode: "M1", WorkflowState: models.CourseAvailable}
	spring := &models.Course{AccountID: 1, Name: "Spring Math", CourseCode: "M2", WorkflowState: models.CourseAvailable}
	for _, c := range []*models.Course{fall, spring} {
		if err := g.Create(c).Error; err != nil {
			t.Fatalf("seed course: %v", err)
		}
	}

	observerEnrollment := func(courseID uint) *models.Enrollment {
		var e models.Enrollment
		err := g.Where("user_id = ? AND course_id = ? AND type = ? AND associated_user_id = ?",
			parent.ID, courseID, "ObserverEnrollment", child.ID).First(&e).Error
		if err != nil {
			return nil
		}
		return &e
	}

	// 1. Pairing in the summer, while the child has no courses, succeeds.
	if err := svc.LinkObserverToStudent(ctx, parent.ID, child.ID); err != nil {
		t.Fatalf("link with no enrollments: %v", err)
	}
	if ok, _ := svc.IsObserverOf(ctx, parent.ID, child.ID); !ok {
		t.Fatal("parent not linked after pairing")
	}

	// 2. The child enrolls in the fall: the parent follows.
	studentFall := &models.Enrollment{UserID: child.ID, CourseID: fall.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive}
	if err := enrollmentRepo.Create(ctx, studentFall); err != nil {
		t.Fatalf("enroll child: %v", err)
	}
	if e := observerEnrollment(fall.ID); e == nil || e.WorkflowState != models.EnrollmentActive {
		t.Fatalf("no active observer enrollment after the child enrolled: %+v", e)
	}

	// 3. The child is dropped: the parent loses access too.
	studentFall.WorkflowState = models.EnrollmentDeleted
	if err := enrollmentRepo.Update(ctx, studentFall); err != nil {
		t.Fatalf("drop child: %v", err)
	}
	if e := observerEnrollment(fall.ID); e == nil || e.WorkflowState != models.EnrollmentDeleted {
		t.Fatalf("observer enrollment not removed when the child was dropped: %+v", e)
	}

	// 4. After unlinking, new courses are no longer mirrored.
	if err := svc.UnlinkObserver(ctx, parent.ID, child.ID); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	studentSpring := &models.Enrollment{UserID: child.ID, CourseID: spring.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive}
	if err := enrollmentRepo.Create(ctx, studentSpring); err != nil {
		t.Fatalf("enroll child in spring: %v", err)
	}
	if e := observerEnrollment(spring.ID); e != nil {
		t.Fatalf("unlinked parent was enrolled in the child's new course: %+v", e)
	}
	if ok, _ := svc.IsObserverOf(ctx, parent.ID, child.ID); ok {
		t.Fatal("parent still linked after unlink")
	}

	// 5. Re-pairing picks up the child's current courses immediately.
	if err := svc.LinkObserverToStudent(ctx, parent.ID, child.ID); err != nil {
		t.Fatalf("re-link: %v", err)
	}
	if e := observerEnrollment(spring.ID); e == nil || e.WorkflowState != models.EnrollmentActive {
		t.Fatalf("re-link did not observe the child's current course: %+v", e)
	}
}
