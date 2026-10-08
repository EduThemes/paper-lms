package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// G112 — OneRoster academicSessions become enrollment terms, classes are
// placed in their term, and roster enrollments land in the class's
// section.

type onerosterAcademicSession struct {
	SourcedID string `json:"sourcedId"`
	Status    string `json:"status"`
	Title     string `json:"title"`
	Type      string `json:"type"` // term, semester, schoolYear, gradingPeriod
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

func (s *OneRosterService) fetchAcademicSessions(baseURL, token string) ([]onerosterAcademicSession, error) {
	raw, err := s.fetchPaginated(baseURL, "/ims/oneroster/v1p1/academicSessions", token, "")
	if err != nil {
		return nil, err
	}
	var out []onerosterAcademicSession
	for _, item := range raw {
		var a onerosterAcademicSession
		if err := json.Unmarshal(item, &a); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// onerosterTermSISID is the SIS term id a roster academic session maps to.
func onerosterTermSISID(sourcedID string) string { return onerosterSISPrefix + sourcedID }

// syncAcademicSessions creates or updates an EnrollmentTerm for every
// term-like academic session (grading periods are not terms). A session
// the SIS marks tobedeleted soft-deletes its term.
func (s *OneRosterService) syncAcademicSessions(ctx context.Context, accountID uint, sessions []onerosterAcademicSession) (int, int, []string) {
	var created, updated int
	var errs []string
	for _, a := range sessions {
		if a.SourcedID == "" || strings.EqualFold(a.Type, "gradingPeriod") {
			continue
		}
		sisID := onerosterTermSISID(a.SourcedID)
		start, err := parseSISDate(a.StartDate)
		if err != nil {
			start = nil
		}
		end, err := parseSISDate(a.EndDate)
		if err != nil {
			end = nil
		}
		state := "active"
		if a.Status == "tobedeleted" {
			state = "deleted"
		}
		var term models.EnrollmentTerm
		err = s.db.WithContext(ctx).Where("account_id = ? AND sis_term_id = ?", accountID, sisID).First(&term).Error
		if err == nil {
			term.Name = SanitizePlainText(a.Title)
			term.StartAt, term.EndAt, term.WorkflowState = start, end, state
			if err := s.db.WithContext(ctx).Save(&term).Error; err != nil {
				errs = append(errs, fmt.Sprintf("failed to update term %s: %v", a.SourcedID, err))
				continue
			}
			updated++
			continue
		}
		if state == "deleted" {
			continue
		}
		term = models.EnrollmentTerm{AccountID: accountID, Name: SanitizePlainText(a.Title), SISTermID: &sisID, StartAt: start, EndAt: end, WorkflowState: state}
		if err := s.db.WithContext(ctx).Create(&term).Error; err != nil {
			errs = append(errs, fmt.Sprintf("failed to create term %s: %v", a.SourcedID, err))
			continue
		}
		created++
	}
	return created, updated, errs
}

// classTermID resolves the term of a class (its first listed academic
// session that maps to a term of the account).
func (s *OneRosterService) classTermID(ctx context.Context, accountID uint, class onerosterClass) *uint {
	for _, t := range class.Terms {
		if t.SourcedID == "" {
			continue
		}
		var term models.EnrollmentTerm
		if err := s.db.WithContext(ctx).
			Where("account_id = ? AND sis_term_id = ? AND COALESCE(workflow_state, 'active') <> 'deleted'", accountID, onerosterTermSISID(t.SourcedID)).
			First(&term).Error; err == nil {
			return &term.ID
		}
	}
	return nil
}

// classSectionID returns the section the sync created for a class.
func (s *OneRosterService) classSectionID(ctx context.Context, courseID uint, classSourcedID string) *uint {
	var sec models.CourseSection
	if err := s.db.WithContext(ctx).
		Where("course_id = ? AND sis_section_id = ?", courseID, "oneroster-section:"+classSourcedID).
		First(&sec).Error; err != nil {
		return nil
	}
	return &sec.ID
}
