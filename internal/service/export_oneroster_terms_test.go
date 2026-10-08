package service

// Test-only exports for the G112 OneRoster term / section tests.

import "context"

// OneRosterTestSession is a flattened academicSession.
type OneRosterTestSession struct {
	SourcedID, Status, Title, Type, StartDate, EndDate string
}

// SyncAcademicSessionsForTest runs syncAcademicSessions.
func (s *OneRosterService) SyncAcademicSessionsForTest(ctx context.Context, accountID uint, rows []OneRosterTestSession) []string {
	sessions := make([]onerosterAcademicSession, len(rows))
	for i, r := range rows {
		sessions[i] = onerosterAcademicSession{SourcedID: r.SourcedID, Status: r.Status, Title: r.Title, Type: r.Type, StartDate: r.StartDate, EndDate: r.EndDate}
	}
	_, _, errs := s.syncAcademicSessions(ctx, accountID, sessions)
	return errs
}

// SyncClassInTermForTest runs syncClasses for one class in a term.
func (s *OneRosterService) SyncClassInTermForTest(ctx context.Context, accountID uint, sourcedID, title, termSourcedID string) []string {
	_, _, errs := s.syncClasses(ctx, accountID, []onerosterClass{{
		SourcedID: sourcedID, Title: title, Status: "active",
		Terms: []onerosterTerm{{SourcedID: termSourcedID, Type: "academicSession"}},
	}})
	return errs
}
