package service_test

// G111 — SIS CSV formats, zip uploads, batch mode, diffing and the change
// threshold. All data is made up.

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func newSISServiceWithObservers(g *gorm.DB) *service.SISImportService {
	svc := newSISService(g)
	obs := service.NewObserverService(postgres.NewEnrollmentRepository(g), postgres.NewCourseRepository(g), postgres.NewUserRepository(g)).
		SetObserverLinkStore(postgres.NewObserverLinkRepository(g))
	return svc.SetObserverLinker(obs)
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func startSIS(t *testing.T, svc *service.SISImportService, accountID uint, opts service.SISImportOptions, uploads ...service.SISUpload) *models.SISBatch {
	t.Helper()
	b, err := svc.StartImport(context.Background(), accountID, 9, opts, uploads)
	if err != nil {
		t.Fatalf("StartImport: %v", err)
	}
	return b
}

func batchErrors(t *testing.T, svc *service.SISImportService, b *models.SISBatch) []string {
	t.Helper()
	errs, _ := svc.GetBatchErrors(context.Background(), b.ID, b.AccountID)
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.File + ": " + e.Message
	}
	return out
}

func TestDetectSISImportType(t *testing.T) {
	cases := map[string]string{
		"user_id,login_id,first_name,last_name,email,status":           "users",
		"term_id,name,status,start_date,end_date":                      "terms",
		"course_id,short_name,long_name,account_id,term_id,status":     "courses",
		"section_id,course_id,name,status":                             "sections",
		"course_id,user_id,role,section_id,status":                     "enrollments",
		"observer_id,student_id,status":                                "user_observers",
		"old_id,new_id,type":                                           "change_sis_id",
		"group_id,group_category_id,course_id,name,status":             "groups",
		"group_category_id,course_id,category_name,status":             "group_categories",
		"group_id,user_id,status":                                      "group_memberships",
		"user_id,login_id,authentication_provider_id,existing_user_id": "logins",
		"user_id,account_id,role,status":                               "admins",
		"account_id,parent_account_id,name,status":                     "accounts",
		"foo,bar": "",
	}
	for header, want := range cases {
		if got := service.DetectSISImportType(strings.Split(header, ",")); got != want {
			t.Errorf("%s → %q, want %q", header, got, want)
		}
	}
}

func TestSISImport_ZipWithEveryFormat(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	svc := newSISServiceWithObservers(g)

	z := zipOf(t, map[string]string{
		"users.csv": "user_id,login_id,first_name,last_name,email,status\n" +
			"U-1,sam.student,Sam,Student,sam@example.com,active\n" +
			"P-1,pat.parent,Pat,Parent,pat@example.com,active\n" +
			"T-1,tia.teacher,Tia,Teacher,tia@example.com,active\n",
		"terms.csv":    "term_id,name,status,start_date,end_date\nFA26,Fall 2026,active,2026-08-20,2026-12-15\n",
		"courses.csv":  "course_id,short_name,long_name,account_id,term_id,status,start_date\nBIO-1,BIO1,Biology,,FA26,active,2026-08-21T08:00:00Z\n",
		"sections.csv": "section_id,course_id,name,status\nBIO-1-A,BIO-1,Period A,active\n",
		// Enrollment named by section only (course_id blank).
		"enrollments.csv":       "course_id,user_id,role,section_id,status\n,U-1,student,BIO-1-A,active\nBIO-1,T-1,teacher,,active\n",
		"user_observers.csv":    "observer_id,student_id,status\nP-1,U-1,active\n",
		"group_categories.csv":  "group_category_id,course_id,category_name,status\nGC-1,BIO-1,Lab Teams,active\n",
		"groups.csv":            "group_id,group_category_id,course_id,name,status\nG-1,GC-1,BIO-1,Team One,available\nG-2,,BIO-1,Team Two,available\n",
		"group_memberships.csv": "group_id,user_id,status\nG-1,U-1,accepted\n",
		"logins.csv":            "user_id,login_id,authentication_provider_id,existing_user_id\nU-1-ALT,sam.alt,,U-1\n",
		"admins.csv":            "user_id,account_id,role,status\nT-1,,AccountAdmin,active\n",
		"change_sis_id.csv":     "old_id,new_id,type\n",
	})
	b := startSIS(t, svc, 1, service.SISImportOptions{}, service.SISUpload{FileName: "feed.zip", Data: z})
	if b.WorkflowState != "imported" {
		t.Fatalf("batch state %q, errors: %v", b.WorkflowState, batchErrors(t, svc, b))
	}

	var course models.Course
	g.Where("sis_course_id = ?", "BIO-1").First(&course)
	var term models.EnrollmentTerm
	g.Where("sis_term_id = ?", "FA26").First(&term)
	if course.EnrollmentTermID == nil || *course.EnrollmentTermID != term.ID || course.StartAt == nil {
		t.Fatalf("courses.csv term_id/start_date ignored: %+v", course)
	}
	var stu models.User
	g.Where("sis_user_id = ?", "U-1").First(&stu)
	var e models.Enrollment
	if err := g.Where("user_id = ? AND course_id = ? AND type = ?", stu.ID, course.ID, "StudentEnrollment").First(&e).Error; err != nil || e.CourseSectionID == nil {
		t.Fatalf("section-only enrollment missing: %v %+v", err, e)
	}
	var parent models.User
	g.Where("sis_user_id = ?", "P-1").First(&parent)
	var obs int64
	g.Model(&models.Enrollment{}).Where("user_id = ? AND course_id = ? AND type = ? AND associated_user_id = ?", parent.ID, course.ID, "ObserverEnrollment", stu.ID).Count(&obs)
	if obs != 1 {
		t.Fatalf("user_observers.csv did not give the parent an associated observer enrollment (%d)", obs)
	}
	var groups []models.Group
	g.Where("context_id = ?", course.ID).Order("id").Find(&groups)
	if len(groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(groups))
	}
	var member int64
	g.Model(&models.GroupMembership{}).Where("group_id = ? AND user_id = ?", groups[0].ID, stu.ID).Count(&member)
	if member != 1 {
		t.Fatal("group membership not created")
	}
	var login models.UserLogin
	if err := g.Where("user_id = ? AND unique_id = ?", stu.ID, "sam.alt").First(&login).Error; err != nil || login.SISUserID == nil || *login.SISUserID != "U-1-ALT" {
		t.Fatalf("logins.csv login missing: %v %+v", err, login)
	}
	var teacher models.User
	g.Where("sis_user_id = ?", "T-1").First(&teacher)
	if teacher.Role != "admin" {
		t.Fatalf("admins.csv did not grant admin: %q", teacher.Role)
	}

	// change_sis_id + user status deleted in a second batch.
	b2 := startSIS(t, svc, 1, service.SISImportOptions{},
		service.SISUpload{FileName: "change_sis_id.csv", Data: []byte("old_id,new_id,type\nBIO-1,BIO-101,course\n")},
		service.SISUpload{FileName: "users.csv", Data: []byte("user_id,login_id,first_name,last_name,email,status\nU-1,sam.student,Sam,Student,sam@example.com,deleted\n")},
	)
	if b2.WorkflowState != "imported" {
		t.Fatalf("batch 2 state %q: %v", b2.WorkflowState, batchErrors(t, svc, b2))
	}
	g.First(&course, course.ID)
	if course.SISCourseID == nil || *course.SISCourseID != "BIO-101" {
		t.Fatalf("change_sis_id did not rename the course: %v", course.SISCourseID)
	}
	g.First(&stu, stu.ID)
	g.First(&e, e.ID)
	if !stu.Suspended || !stu.SuspendedBySIS || e.WorkflowState != models.EnrollmentDeleted {
		t.Fatalf("status=deleted user not suspended / enrollments not removed: %+v / %s", stu.Suspended, e.WorkflowState)
	}
	g.Model(&models.Enrollment{}).Where("user_id = ? AND course_id = ? AND type = ? AND workflow_state = ?", parent.ID, course.ID, "ObserverEnrollment", "active").Count(&obs)
	if obs != 0 {
		t.Fatal("the observer kept access to a deleted student's course")
	}
}

func TestSISImport_TenantScopeOfNewFormats(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	svc := newSISServiceWithObservers(g)

	startSIS(t, svc, 1, service.SISImportOptions{}, service.SISUpload{FileName: "users.csv",
		Data: []byte("user_id,login_id,first_name,last_name,email\nA-1,alex.one,Alex,One,alex@example.com\n")})
	b := startSIS(t, svc, 2, service.SISImportOptions{},
		service.SISUpload{FileName: "admins.csv", Data: []byte("user_id,account_id,role,status\nA-1,,AccountAdmin,active\n")},
		service.SISUpload{FileName: "change_sis_id.csv", Data: []byte("old_id,new_id,type\nA-1,A-9,user\n")},
		service.SISUpload{FileName: "accounts.csv", Data: []byte("account_id,parent_account_id,name,status\n1,,Hijacked,active\n")},
	)
	if got := len(batchErrors(t, svc, b)); got != 3 {
		t.Fatalf("want 3 refused rows, got %d: %v", got, batchErrors(t, svc, b))
	}
	var u models.User
	g.Where("login_id = ?", "alex.one").First(&u)
	if u.Role == "admin" || u.SISUserID == nil || *u.SISUserID != "A-1" {
		t.Fatalf("tenant 2 changed tenant 1's user: %+v", u)
	}
	var acct models.Account
	g.First(&acct, 1)
	if acct.Name == "Hijacked" {
		t.Fatal("tenant 2 renamed tenant 1")
	}
}

func TestSISImport_BatchModeAndThreshold(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	svc := newSISServiceWithObservers(g)
	setup := "term_id,name\nSP27,Spring 2027\n"
	startSIS(t, svc, 1, service.SISImportOptions{}, service.SISUpload{FileName: "terms.csv", Data: []byte(setup)})
	var term models.EnrollmentTerm
	g.Where("sis_term_id = ?", "SP27").First(&term)

	courses := "course_id,short_name,long_name,term_id,status\nM-1,M1,Math 1,SP27,active\nM-2,M2,Math 2,SP27,active\nM-3,M3,Math 3,SP27,active\nM-4,M4,Math 4,SP27,active\n"
	startSIS(t, svc, 1, service.SISImportOptions{}, service.SISUpload{FileName: "courses.csv", Data: []byte(courses)})
	// A manually created (non-SIS) course in the term is never touched.
	manual := &models.Course{AccountID: 1, Name: "Manual", CourseCode: "MAN", WorkflowState: models.CourseAvailable, EnrollmentTermID: &term.ID}
	g.Create(manual)

	// A batch-mode import without courses.csv never deletes courses.
	b0 := startSIS(t, svc, 1, service.SISImportOptions{BatchMode: true, BatchModeTermID: &term.ID},
		service.SISUpload{FileName: "users.csv", Data: []byte("user_id,login_id,first_name,last_name,email\nBM-1,bm.one,Bo,One,bo@example.com\n")})
	if b0.WorkflowState != "imported" {
		t.Fatalf("users-only batch state %q: %v", b0.WorkflowState, batchErrors(t, svc, b0))
	}
	var gone int64
	g.Model(&models.Course{}).Where("workflow_state = ?", "deleted").Count(&gone)
	if gone != 0 {
		t.Fatalf("a batch without courses.csv deleted %d courses", gone)
	}

	// Threshold 25%: dropping 3 of 4 is refused, nothing deleted.
	th := 25
	b := startSIS(t, svc, 1, service.SISImportOptions{BatchMode: true, BatchModeTermID: &term.ID, ChangeThreshold: &th},
		service.SISUpload{FileName: "courses.csv", Data: []byte("course_id,short_name,long_name,term_id,status\nM-1,M1,Math 1,SP27,active\n")})
	if b.WorkflowState != "aborted" {
		t.Fatalf("want aborted, got %q", b.WorkflowState)
	}
	var deleted int64
	g.Model(&models.Course{}).Where("workflow_state = ?", "deleted").Count(&deleted)
	if deleted != 0 {
		t.Fatalf("aborted batch deleted %d courses", deleted)
	}

	// Without a threshold, batch mode deletes the three unmentioned SIS courses.
	b = startSIS(t, svc, 1, service.SISImportOptions{BatchMode: true, BatchModeTermID: &term.ID},
		service.SISUpload{FileName: "courses.csv", Data: []byte("course_id,short_name,long_name,term_id,status\nM-1,M1,Math 1,SP27,active\n")})
	if b.WorkflowState != "imported" {
		t.Fatalf("batch state %q: %v", b.WorkflowState, batchErrors(t, svc, b))
	}
	g.Model(&models.Course{}).Where("workflow_state = ? AND sis_course_id IN ?", "deleted", []string{"M-2", "M-3", "M-4"}).Count(&deleted)
	if deleted != 3 {
		t.Fatalf("batch mode deleted %d of the 3 unmentioned courses", deleted)
	}
	g.First(manual, manual.ID)
	if manual.WorkflowState == models.CourseDeleted {
		t.Fatal("batch mode deleted a non-SIS course")
	}

	// Batch mode needs a term of this account.
	other := uint(999999)
	if _, err := svc.StartImport(context.Background(), 1, 9, service.SISImportOptions{BatchMode: true, BatchModeTermID: &other},
		[]service.SISUpload{{FileName: "courses.csv", Data: []byte(courses)}}); err == nil {
		t.Fatal("batch mode accepted a foreign term")
	}
}

func TestSISImport_Diffing(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	svc := newSISServiceWithObservers(g)
	opts := service.SISImportOptions{DiffingDataSetIdentifier: "nightly"}

	first := "user_id,login_id,first_name,last_name,email,status\nD-1,dana,Dana,One,dana@example.com,active\nD-2,drew,Drew,Two,drew@example.com,active\n"
	b1 := startSIS(t, svc, 1, opts, service.SISUpload{FileName: "users.csv", Data: []byte(first)})
	if b1.ProcessedRows != 2 {
		t.Fatalf("first batch processed %d rows, want 2", b1.ProcessedRows)
	}
	// D-1 unchanged, D-2 dropped from the feed, D-3 new.
	second := "user_id,login_id,first_name,last_name,email,status\nD-1,dana,Dana,One,dana@example.com,active\nD-3,dale,Dale,Three,dale@example.com,active\n"
	b2 := startSIS(t, svc, 1, opts, service.SISUpload{FileName: "users.csv", Data: []byte(second)})
	if b2.DiffedAgainstBatchID == nil || *b2.DiffedAgainstBatchID != b1.ID {
		t.Fatalf("batch 2 not diffed against batch 1: %v", b2.DiffedAgainstBatchID)
	}
	if b2.ProcessedRows != 2 { // D-3 created + D-2 deleted; D-1 skipped
		t.Fatalf("diffed batch processed %d rows, want 2", b2.ProcessedRows)
	}
	var drew models.User
	g.Where("sis_user_id = ?", "D-2").First(&drew)
	if !drew.Suspended {
		t.Fatal("a user missing from the diffed feed was not removed")
	}

	// A threshold stops a diff that would delete too much.
	th := 10
	opts.ChangeThreshold = &th
	b3 := startSIS(t, svc, 1, opts, service.SISUpload{FileName: "users.csv", Data: []byte("user_id,login_id,first_name,last_name,email,status\nD-9,dot,Dot,Nine,dot@example.com,active\n")})
	if b3.WorkflowState != "aborted" {
		t.Fatalf("want aborted diff, got %q", b3.WorkflowState)
	}
}
