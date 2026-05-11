package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// OutcomeProficiencyService manages proficiency scales for accounts and courses
// and computes the proficiency level for a given score.
type OutcomeProficiencyService struct {
	repo *postgres.OutcomeProficiencyRepository
}

func NewOutcomeProficiencyService(repo *postgres.OutcomeProficiencyRepository) *OutcomeProficiencyService {
	return &OutcomeProficiencyService{repo: repo}
}

// Get returns the proficiency for the given context, or the resolved fallback
// (account -> system default) when context is a Course. Returns the system
// default when nothing exists for an Account.
func (s *OutcomeProficiencyService) Get(ctx context.Context, contextType string, contextID uint) (*models.OutcomeProficiency, error) {
	switch contextType {
	case "Course":
		return s.repo.ResolveForCourse(ctx, contextID)
	case "Account":
		p, err := s.repo.FindByContext(ctx, "Account", contextID)
		if err == nil {
			return p, nil
		}
		return &models.OutcomeProficiency{
			ContextType:   "System",
			WorkflowState: "active",
			Ratings:       models.DefaultProficiencyRatings(),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported context type: %s", contextType)
	}
}

// Set creates or replaces the proficiency scale for the given context.
func (s *OutcomeProficiencyService) Set(ctx context.Context, contextType string, contextID uint, ratings []models.OutcomeProficiencyRating) (*models.OutcomeProficiency, error) {
	if contextType != "Course" && contextType != "Account" {
		return nil, fmt.Errorf("unsupported context type: %s", contextType)
	}
	if len(ratings) < 2 {
		return nil, errors.New("a proficiency scale must have at least 2 ratings")
	}
	masteryCount := 0
	for _, r := range ratings {
		if r.Description == "" {
			return nil, errors.New("each rating must have a description")
		}
		if r.Mastery {
			masteryCount++
		}
	}
	if masteryCount != 1 {
		return nil, errors.New("exactly one rating must be marked as mastery")
	}
	return s.repo.Upsert(ctx, contextType, contextID, ratings)
}

// Reset removes the context-specific scale (so resolution falls through to the
// parent / system default).
func (s *OutcomeProficiencyService) Reset(ctx context.Context, contextType string, contextID uint) error {
	return s.repo.Delete(ctx, contextType, contextID)
}

// LevelFor returns the rating that applies to a given score against the supplied
// proficiency scale. The matching rule mirrors Canvas: the highest-points rating
// whose Points threshold is <= score. If the score is below the lowest threshold,
// the lowest rating is returned. Returns nil if the scale has no ratings.
func (s *OutcomeProficiencyService) LevelFor(score float64, p *models.OutcomeProficiency) *models.OutcomeProficiencyRating {
	if p == nil || len(p.Ratings) == 0 {
		return nil
	}
	var best *models.OutcomeProficiencyRating
	var lowest *models.OutcomeProficiencyRating
	for i := range p.Ratings {
		r := &p.Ratings[i]
		if lowest == nil || r.Points < lowest.Points {
			lowest = r
		}
		if score >= r.Points {
			if best == nil || r.Points > best.Points {
				best = r
			}
		}
	}
	if best != nil {
		return best
	}
	return lowest
}

// LevelForCourse fetches the resolved scale for a course and returns the rating
// for the given score in one call.
func (s *OutcomeProficiencyService) LevelForCourse(ctx context.Context, courseID uint, score float64) (*models.OutcomeProficiencyRating, *models.OutcomeProficiency, error) {
	p, err := s.repo.ResolveForCourse(ctx, courseID)
	if err != nil {
		return nil, nil, err
	}
	return s.LevelFor(score, p), p, nil
}
