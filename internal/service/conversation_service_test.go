package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// Private messages render as rich content, so an unsanitized body was a
// stored-XSS vector. CreateMessage is the single send chokepoint and must
// sanitize regardless of caller.
func TestConversationCreateMessage_SanitizesBody(t *testing.T) {
	msgRepo := new(mocks.MockConversationMessageRepository)
	convRepo := new(mocks.MockConversationRepository)
	msgRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	convRepo.On("FindByID", mock.Anything, mock.Anything, mock.Anything).
		Return(&models.Conversation{ID: 1}, nil)
	convRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	svc := service.NewConversationService(convRepo, nil, msgRepo)

	msg := &models.ConversationMessage{
		ConversationID: 1,
		UserID:         2,
		Body:           `<script>alert('xss')</script><b>Hi</b>`,
	}
	require.NoError(t, svc.CreateMessage(context.Background(), msg))

	assert.NotContains(t, msg.Body, "<script>", "script tag must be stripped from DM body")
	assert.Contains(t, msg.Body, "Hi", "legitimate text must survive sanitization")
}
