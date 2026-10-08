package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// SIS support for cross-listing and enrollment / section dates
// (admin-enrollments lane, G106 / G108).
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

// sisDates parses the optional start_date / end_date columns.
func sisDates(record []string, colIndex map[string]int) (*time.Time, *time.Time, error) {
	parse := func(col string) (*time.Time, error) {
		raw := getColumn(record, colIndex, col)
		if raw == "" {
			return nil, nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, raw); err == nil {
				return &t, nil
			}
		}
		return nil, fmt.Errorf("invalid %s '%s' (expected ISO 8601)", col, raw)
	}
	start, err := parse("start_date")
	if err != nil {
		return nil, nil, err
	}
	end, err := parse("end_date")
	if err != nil {
		return nil, nil, err
	}
	if start != nil && end != nil && end.Before(*start) {
		return nil, nil, fmt.Errorf("end_date must be after start_date")
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

func (s *SISImportService) processXlistsCSV(ctx context.Context, batchID, accountID uint, reader *csv.Reader) error {
	if s.sectionService == nil {
		return fmt.Errorf("cross-listing is not configured")
	}
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}
	colIndex := buildColumnIndex(header)

	batch, _ := s.batchRepo.FindByID(ctx, batchID)
	rowNum := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to read row: %v", err), "xlists.csv")
			rowNum++
			continue
		}
		rowNum++
		batch.TotalRows = rowNum - 1

		sisCourseID := getColumn(record, colIndex, "xlist_course_id")
		sisSectionID := getColumn(record, colIndex, "section_id")
		status := strings.ToLower(getColumn(record, colIndex, "status"))
		if sisCourseID == "" || sisSectionID == "" {
			s.recordError(ctx, batchID, rowNum, "xlist_course_id and section_id are required", "xlists.csv")
			continue
		}
		if status != "active" && status != "deleted" {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("invalid status '%s' (expected active or deleted)", status), "xlists.csv")
			continue
		}
		// Both lookups are confined to the batch's account.
		course, findErr := s.findCourseBySISID(ctx, sisCourseID, accountID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("course with sis_course_id '%s' not found", sisCourseID), "xlists.csv")
			continue
		}
		section, findErr := s.findSectionBySISID(ctx, sisSectionID, accountID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("section with sis_section_id '%s' not found", sisSectionID), "xlists.csv")
			continue
		}
		if err := s.sectionService.CrosslistBySIS(ctx, section, course, status == "active"); err != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("could not cross-list section '%s': %v", sisSectionID, err), "xlists.csv")
			continue
		}

		batch.ProcessedRows++
		if batch.TotalRows > 0 {
			batch.Progress = (batch.ProcessedRows * 100) / batch.TotalRows
		}
		s.batchRepo.Update(ctx, batch)
	}
	batch.TotalRows = rowNum - 1
	s.batchRepo.Update(ctx, batch)
	return nil
}
