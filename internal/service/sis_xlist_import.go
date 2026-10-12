package service

import (
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// SIS support for cross-listing and enrollment / section dates
// (admin-enrollments lane, G106 / G108), on the batch runner (G111).
//
// xlists.csv (Canvas format): xlist_course_id, section_id, status
//   - status "active" cross-lists the section into the course;
//   - status "deleted" de-cross-lists it (back to its original course).
//
// sections.csv and enrollments.csv also accept start_date / end_date
// (ISO 8601), and enrollments.csv limit_section_privileges (true/false).

// SetSectionService wires the cross-listing service used by xlists.csv.
// Returns the service for chaining.
func (s *SISImportService) SetSectionService(sections *SectionService) *SISImportService {
	s.sectionService = sections
	return s
}

// sisRowDates parses a row's optional start_date / end_date columns.
func sisRowDates(row sisRow) (*time.Time, *time.Time, error) {
	start, err := parseSISDate(row.get("start_date"))
	if err != nil {
		return nil, nil, rowErr("invalid start_date: %v", err)
	}
	end, err := parseSISDate(row.get("end_date"))
	if err != nil {
		return nil, nil, rowErr("invalid end_date: %v", err)
	}
	if start != nil && end != nil && end.Before(*start) {
		return nil, nil, errors.New("end_date must be after start_date")
	}
	return start, end, nil
}

// applySISSectionDates sets section dates from a sections.csv row. Dates
// on a section only bound enrollments when restricted, as in Canvas's SIS
// import, so a row carrying dates turns the restriction on.
func applySISSectionDates(section *models.CourseSection, start, end *time.Time) {
	if start == nil && end == nil {
		return
	}
	section.StartAt = start
	section.EndAt = end
	section.RestrictEnrollmentsToSectionDates = true
}

// importXlist applies one xlists.csv row. Both lookups are confined to
// the batch's account.
func (r *sisRun) importXlist(row sisRow) error {
	if r.s.sectionService == nil {
		return errors.New("cross-listing is not configured")
	}
	sisCourseID := row.get("xlist_course_id")
	sisSectionID := row.get("section_id")
	status := strings.ToLower(row.get("status"))
	if sisCourseID == "" || sisSectionID == "" {
		return rowErr("xlist_course_id and section_id are required")
	}
	if status != "active" && status != "deleted" {
		return rowErr("invalid status '%s' (expected active or deleted)", status)
	}
	course, err := r.s.findCourseBySISID(r.ctx, sisCourseID, r.accountID)
	if err != nil {
		return rowErr("course with sis_course_id '%s' not found", sisCourseID)
	}
	section, err := r.s.findSectionBySISID(r.ctx, sisSectionID, r.accountID)
	if err != nil {
		return rowErr("section with sis_section_id '%s' not found", sisSectionID)
	}
	if err := r.s.sectionService.CrosslistBySIS(r.ctx, section, course, status == "active"); err != nil {
		return rowErr("could not cross-list section '%s': %v", sisSectionID, err)
	}
	return nil
}
