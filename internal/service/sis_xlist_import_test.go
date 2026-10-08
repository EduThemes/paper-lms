package service_test

// SIS cross-listing (xlists.csv, G108) and SIS enrollment / section dates
// (G106). All data is made up.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func TestSISImport_XlistsAndDates(t *testing.T) {
	g, cleanup := freshDB(t)
	defer cleanup()
	seedSISTenants(t, g)
	ctx := context.Background()

	adminRepo := postgres.NewEnrollmentAdminRepository(g)
	sections := service.NewSectionService(postgres.NewSectionAdminRepository(g), adminRepo, postgres.NewCourseRepository(g))
	svc := newSISService(g).SetSectionService(sections)

	runSIS(t, svc, 1, "courses", "course_id,short_name,long_name,status\nBIO-101,BIO,Biology,active\nBIO-201,BIO2,Biology II,active\n")
	runSIS(t, svc, 2, "courses", "course_id,short_name,long_name,status\nOTHER-1,OTH,Other,active\n")
	runSIS(t, svc, 1, "users", "user_id,login_id,first_name,last_name,email,status\nU-1,sam.sis,Sam,Student,sam.sis@example.test,active\n")
	runSIS(t, svc, 1, "sections", "section_id,course_id,name,status,start_date,end_date\nSEC-1,BIO-101,Morning,active,2026-01-10,2026-05-10\n")
	runSIS(t, svc, 1, "enrollments", "course_id,user_id,role,section_id,status,start_date,end_date,limit_section_privileges\nBIO-101,U-1,student,SEC-1,active,2026-01-15T00:00:00Z,2026-05-01T00:00:00Z,true\n")

	var sec models.CourseSection
	require.NoError(t, g.Where("sis_section_id = ?", "SEC-1").First(&sec).Error)
	assert.True(t, sec.RestrictEnrollmentsToSectionDates, "SIS section dates restrict participation")
	require.NotNil(t, sec.StartAt)
	var e models.Enrollment
	require.NoError(t, g.Where("course_section_id = ?", sec.ID).First(&e).Error)
	require.NotNil(t, e.EndAt)
	assert.True(t, e.LimitPrivilegesToCourseSection)

	var bio, bio2 models.Course
	require.NoError(t, g.Where("sis_course_id = ?", "BIO-101").First(&bio).Error)
	require.NoError(t, g.Where("sis_course_id = ?", "BIO-201").First(&bio2).Error)

	// Cross-list into BIO-201.
	runSIS(t, svc, 1, "xlists", "xlist_course_id,section_id,status\nBIO-201,SEC-1,active\n")
	require.NoError(t, g.First(&sec, sec.ID).Error)
	assert.Equal(t, bio2.ID, sec.CourseID)
	require.NotNil(t, sec.NonxlistCourseID)
	assert.Equal(t, bio.ID, *sec.NonxlistCourseID)
	require.NoError(t, g.First(&e, e.ID).Error)
	assert.Equal(t, bio2.ID, e.CourseID, "enrollments follow the cross-listed section")

	// A sections.csv re-import keeps the cross-listing (home course stays BIO-101).
	runSIS(t, svc, 1, "sections", "section_id,course_id,name,status\nSEC-1,BIO-101,Morning (renamed),active\n")
	require.NoError(t, g.First(&sec, sec.ID).Error)
	assert.Equal(t, bio2.ID, sec.CourseID)
	assert.Equal(t, "Morning (renamed)", sec.Name)

	// Another tenant's course can't be a cross-list target.
	batchID := runSIS(t, svc, 1, "xlists", "xlist_course_id,section_id,status\nOTHER-1,SEC-1,active\n")
	errs, err := svc.GetBatchErrors(ctx, batchID, 1)
	require.NoError(t, err)
	assert.Len(t, errs, 1)

	// De-cross-list.
	runSIS(t, svc, 1, "xlists", "xlist_course_id,section_id,status\nBIO-201,SEC-1,deleted\n")
	require.NoError(t, g.First(&sec, sec.ID).Error)
	assert.Equal(t, bio.ID, sec.CourseID)
	assert.Nil(t, sec.NonxlistCourseID)
	require.NoError(t, g.First(&e, e.ID).Error)
	assert.Equal(t, bio.ID, e.CourseID)

	// Bad dates are row errors.
	batchID = runSIS(t, svc, 1, "enrollments", "course_id,user_id,role,status,start_date\nBIO-101,U-1,ta,active,not-a-date\n")
	errs, err = svc.GetBatchErrors(ctx, batchID, 1)
	require.NoError(t, err)
	assert.Len(t, errs, 1)
}
