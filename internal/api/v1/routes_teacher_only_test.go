// Route-wiring guard for B178 / B179.
//
// DELETE /courses/:id and the course audit log (JSON + CSV) must be
// gated by RequireCourseRole("TeacherEnrollment"), not the `instructor`
// gate (teacher OR TA): a TA must not soft-delete the course (there is no
// restore) or read every user's activity trail. Register() needs every
// handler wired, so this pins the mount lines in router.go instead; the
// gate's behavior itself is covered in handlers/audit_redaction_test.go.
package v1

import (
	"os"
	"strings"
	"testing"
)

func TestTeacherOnlyRoutesAreNotInstructorGated(t *testing.T) {
	src, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	mounts := map[string]string{
		`protected.Delete("/courses/:id",`:                   "DELETE /courses/:id",
		`protected.Get("/courses/:course_id/audit_log",`:     "GET /courses/:course_id/audit_log",
		`protected.Get("/courses/:course_id/audit_log.csv",`: "GET /courses/:course_id/audit_log.csv",
	}
	for prefix, name := range mounts {
		found := false
		for _, line := range strings.Split(string(src), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			found = true
			if !strings.Contains(line, `RequireCourseRole("TeacherEnrollment")`) {
				t.Errorf("%s must be gated by RequireCourseRole(\"TeacherEnrollment\"), got: %s", name, line)
			}
		}
		if !found {
			t.Errorf("%s mount not found in router.go", name)
		}
	}
}
