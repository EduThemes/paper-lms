package service

import (
	"context"
	"errors"
	"strings"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type CommentBankService struct {
	repo repository.CommentBankItemRepository
}

func NewCommentBankService(repo repository.CommentBankItemRepository) *CommentBankService {
	return &CommentBankService{repo: repo}
}

func (s *CommentBankService) Create(ctx context.Context, userID uint, item *models.CommentBankItem) error {
	if strings.TrimSpace(item.Comment) == "" {
		return errors.New("comment is required")
	}
	item.UserID = userID
	return s.repo.Create(ctx, item)
}

func (s *CommentBankService) Get(ctx context.Context, userID uint, id uint) (*models.CommentBankItem, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("comment bank item not found")
	}
	if item.UserID != userID {
		return nil, errors.New("unauthorized")
	}
	return item, nil
}

func (s *CommentBankService) Update(ctx context.Context, userID uint, id uint, comment string) (*models.CommentBankItem, error) {
	if strings.TrimSpace(comment) == "" {
		return nil, errors.New("comment is required")
	}

	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("comment bank item not found")
	}
	if item.UserID != userID {
		return nil, errors.New("unauthorized")
	}

	item.Comment = comment
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *CommentBankService) Delete(ctx context.Context, userID uint, id uint) error {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("comment bank item not found")
	}
	if item.UserID != userID {
		return errors.New("unauthorized")
	}
	return s.repo.Delete(ctx, id)
}

func (s *CommentBankService) List(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CommentBankItem], error) {
	return s.repo.ListByUserID(ctx, userID, params)
}

func (s *CommentBankService) Search(ctx context.Context, userID uint, query string) ([]models.CommentBankItem, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("search_term is required")
	}
	return s.repo.SearchByUser(ctx, userID, query)
}
