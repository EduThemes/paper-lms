package service

// Test-only exports for the OneRoster sync integration tests in
// oneroster_sync_test.go (package service_test).

import (
	"context"
)

// OneRosterTestUser / OneRosterTestEnrollment are flattened roster rows.
type OneRosterTestUser struct {
	SourcedID, Status, Role, GivenName, FamilyName, Username string
	Agents                                                   []string
}

type OneRosterTestEnrollment struct {
	SourcedID, Status, Role, UserSourcedID, ClassSourcedID string
}

type OneRosterTestClass struct {
	SourcedID, Title string
}

// SyncUsersForTest runs syncUsers on flattened roster rows.
func (s *OneRosterService) SyncUsersForTest(ctx context.Context, accountID uint, rows []OneRosterTestUser) []string {
	users := make([]onerosterUser, len(rows))
	for i, r := range rows {
		u := onerosterUser{SourcedID: r.SourcedID, Status: r.Status, Role: r.Role, GivenName: r.GivenName, FamilyName: r.FamilyName, Username: r.Username}
		for _, a := range r.Agents {
			u.Agents = append(u.Agents, onerosterOrgRef{SourcedID: a, Type: "user"})
		}
		users[i] = u
	}
	_, _, errs := s.syncUsers(ctx, accountID, users)
	return errs
}

// SyncClassesForTest runs syncClasses on flattened roster rows.
func (s *OneRosterService) SyncClassesForTest(ctx context.Context, accountID uint, rows []OneRosterTestClass) []string {
	classes := make([]onerosterClass, len(rows))
	for i, r := range rows {
		classes[i] = onerosterClass{SourcedID: r.SourcedID, Title: r.Title, Status: "active"}
	}
	_, _, errs := s.syncClasses(ctx, accountID, classes)
	return errs
}

// SyncEnrollmentsForTest runs syncEnrollments and, when removeAbsent is
// set, the full-sync absence pass. Returns errors and the abort reason.
func (s *OneRosterService) SyncEnrollmentsForTest(ctx context.Context, accountID uint, rows []OneRosterTestEnrollment, removeAbsent bool) ([]string, string) {
	enrollments := make([]onerosterEnrollment, len(rows))
	for i, r := range rows {
		enrollments[i] = onerosterEnrollment{
			SourcedID: r.SourcedID, Status: r.Status, Role: r.Role,
			User:  onerosterOrgRef{SourcedID: r.UserSourcedID},
			Class: onerosterOrgRef{SourcedID: r.ClassSourcedID},
		}
	}
	res := s.syncEnrollments(ctx, accountID, enrollments)
	errs := res.errs
	reason := ""
	if removeAbsent {
		_, abortReason, rmErrs := s.removeAbsentEnrollments(ctx, accountID, res.present)
		errs = append(errs, rmErrs...)
		reason = abortReason
	}
	return errs, reason
}
