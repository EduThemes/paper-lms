package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// CommunicationChannelRepository defines the data access methods for communication channels.
type CommunicationChannelRepository interface {
	Create(ctx context.Context, channel *models.CommunicationChannel) error
	FindByID(ctx context.Context, id uint) (*models.CommunicationChannel, error)
	Update(ctx context.Context, channel *models.CommunicationChannel) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint) ([]models.CommunicationChannel, error)
	FindByUserIDAndType(ctx context.Context, userID uint, channelType string) ([]models.CommunicationChannel, error)
	FindPrimaryByUserID(ctx context.Context, userID uint) (*models.CommunicationChannel, error)
}

type communicationChannelRepo struct {
	db *gorm.DB
}

// NewCommunicationChannelRepository creates a new CommunicationChannelRepository backed by PostgreSQL.
func NewCommunicationChannelRepository(db *gorm.DB) CommunicationChannelRepository {
	return &communicationChannelRepo{db: db}
}

func (r *communicationChannelRepo) Create(ctx context.Context, channel *models.CommunicationChannel) error {
	return r.db.WithContext(ctx).Create(channel).Error
}

func (r *communicationChannelRepo) FindByID(ctx context.Context, id uint) (*models.CommunicationChannel, error) {
	var channel models.CommunicationChannel
	if err := r.db.WithContext(ctx).First(&channel, id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *communicationChannelRepo) Update(ctx context.Context, channel *models.CommunicationChannel) error {
	return r.db.WithContext(ctx).Save(channel).Error
}

func (r *communicationChannelRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.CommunicationChannel{}).
		Where("id = ?", id).
		Update("workflow_state", "retired").Error
}

func (r *communicationChannelRepo) ListByUserID(ctx context.Context, userID uint) ([]models.CommunicationChannel, error) {
	var channels []models.CommunicationChannel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND workflow_state = ?", userID, "active").
		Order("position ASC").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

func (r *communicationChannelRepo) FindByUserIDAndType(ctx context.Context, userID uint, channelType string) ([]models.CommunicationChannel, error) {
	var channels []models.CommunicationChannel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND channel_type = ? AND workflow_state = ?", userID, channelType, "active").
		Order("position ASC").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// FindPrimaryByUserID returns the channel with the lowest position for the given user.
func (r *communicationChannelRepo) FindPrimaryByUserID(ctx context.Context, userID uint) (*models.CommunicationChannel, error) {
	var channel models.CommunicationChannel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND workflow_state = ?", userID, "active").
		Order("position ASC").
		First(&channel).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}
