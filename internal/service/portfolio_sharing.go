package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sort"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ePortfolio additions (G155): private sharing links, comment moderation,
// and picking course submissions to add.

// CreateShareLink gives the portfolio a new secret sharing token (replacing
// any previous one, which stops working) and returns it. Like Canvas's
// private ePortfolio link, anyone holding the link can view the portfolio
// without it being public.
func (s *PortfolioService) CreateShareLink(ctx context.Context, portfolio *models.Portfolio) (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	portfolio.ShareToken = &token
	portfolio.UpdatedAt = time.Now()
	if err := s.portfolioRepo.Update(ctx, portfolio); err != nil {
		return "", err
	}
	return token, nil
}

// RevokeShareLink removes the portfolio's sharing link.
func (s *PortfolioService) RevokeShareLink(ctx context.Context, portfolio *models.Portfolio) error {
	portfolio.ShareToken = nil
	return s.portfolioRepo.Update(ctx, portfolio)
}

// GetBySharedLink resolves a sharing token. Archived portfolios stay
// reachable (they are read-only); deleted ones are not found.
func (s *PortfolioService) GetBySharedLink(ctx context.Context, token string) (*models.Portfolio, error) {
	if len(token) < 16 {
		return nil, errors.New("portfolio not found")
	}
	return s.portfolioRepo.FindByShareToken(ctx, token)
}

// SetCommentsDisabled turns viewer comments on or off.
func (s *PortfolioService) SetCommentsDisabled(ctx context.Context, portfolio *models.Portfolio, disabled bool) error {
	portfolio.CommentsDisabled = disabled
	return s.portfolioRepo.Update(ctx, portfolio)
}

// GetComment loads a comment of the portfolio (tenant-scoped); a comment of
// another portfolio is ErrPortfolioChildNotFound.
func (s *PortfolioService) GetComment(ctx context.Context, portfolioID, commentID, accountID uint) (*models.PortfolioComment, error) {
	comment, err := s.commentRepo.FindByID(ctx, commentID, accountID)
	if err != nil || comment == nil || comment.PortfolioID != portfolioID {
		return nil, ErrPortfolioChildNotFound
	}
	return comment, nil
}

// DeleteComment removes a comment of the portfolio.
func (s *PortfolioService) DeleteComment(ctx context.Context, portfolioID, commentID, accountID uint) error {
	if _, err := s.GetComment(ctx, portfolioID, commentID, accountID); err != nil {
		return err
	}
	return s.commentRepo.Delete(ctx, commentID)
}

// ImportableSubmission is one of the owner's turned-in submissions that can
// be added to the portfolio.
type ImportableSubmission struct {
	SubmissionID    uint       `json:"submission_id"`
	AssignmentID    uint       `json:"assignment_id"`
	AssignmentName  string     `json:"assignment_name"`
	CourseID        uint       `json:"course_id"`
	SubmissionType  string     `json:"submission_type"`
	SubmittedAt     *time.Time `json:"submitted_at"`
	FileCount       int        `json:"file_count"`
	HasText         bool       `json:"has_text"`
	URL             string     `json:"url,omitempty"`
	AlreadyImported bool       `json:"already_imported"`
}

// ListImportableSubmissions lists the portfolio owner's turned-in
// submissions in a course, newest first, marking those already in the
// portfolio. Only the owner's own submissions are ever listed.
func (s *PortfolioService) ListImportableSubmissions(ctx context.Context, portfolio *models.Portfolio, courseID, accountID uint) ([]ImportableSubmission, error) {
	subs, err := s.submissionRepo.ListByUserAndCourse(ctx, portfolio.UserID, courseID)
	if err != nil {
		return nil, err
	}
	imported := s.importedSubmissionIDs(ctx, portfolio.ID)
	out := []ImportableSubmission{}
	for i := range subs {
		sub := &subs[i]
		if sub.SubmittedAt == nil || sub.UserID != portfolio.UserID {
			continue
		}
		assignment, err := s.assignmentRepo.FindByID(ctx, sub.AssignmentID, accountID)
		if err != nil || assignment == nil || assignment.CourseID != courseID {
			continue
		}
		item := ImportableSubmission{
			SubmissionID:    sub.ID,
			AssignmentID:    assignment.ID,
			AssignmentName:  assignment.Name,
			CourseID:        assignment.CourseID,
			SubmittedAt:     sub.SubmittedAt,
			FileCount:       len(submissionFiles(sub)),
			HasText:         sub.Body != nil && SanitizePlainText(*sub.Body) != "",
			AlreadyImported: imported[sub.ID],
		}
		if sub.SubmissionType != nil {
			item.SubmissionType = *sub.SubmissionType
		}
		if sub.URL != nil && isWebURL(*sub.URL) {
			item.URL = *sub.URL
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SubmittedAt.After(*out[j].SubmittedAt) })
	return out, nil
}
