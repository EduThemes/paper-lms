package service

import (
	"context"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// TestOneRosterSyncUsers_NewUser_PasswordNotDerivedFromSourcedID locks
// in the security contract the prior code violated: the initial
// password hash for an SIS-provisioned user MUST NOT be derivable
// from the user's public SourcedID.
//
// Prior to the fix, the call site was:
//
//	newUser.HashPassword("OneRoster-" + orUser.SourcedID + "-changeme")
//
// — meaning any attacker who knew the SIS sourcedId of a user could
// compute their bcrypt input and log in. This test runs syncUsers
// against a captured-in-memory user, then verifies bcrypt.Compare
// against the predictable string REJECTS the stored hash.
func TestOneRosterSyncUsers_NewUser_PasswordNotDerivedFromSourcedID(t *testing.T) {
	mockUserRepo := new(mocks.MockUserRepository)
	svc := &OneRosterService{userRepo: mockUserRepo}

	const sourcedID = "user-abc-123"

	// FindBySISUserID returns nil = no existing user, triggering the
	// Create branch.
	mockUserRepo.
		On("FindBySISUserID", mock.Anything, "oneroster:"+sourcedID).
		Return(nil, nil)

	// Capture the user passed to Create so we can inspect the hash.
	var captured *models.User
	mockUserRepo.
		On("Create", mock.Anything, mock.AnythingOfType("*models.User")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(*models.User)
		}).
		Return(nil)

	created, _, errs := svc.syncUsers(context.Background(), []onerosterUser{
		{
			SourcedID:  sourcedID,
			Status:     "active",
			GivenName:  "Test",
			FamilyName: "User",
			Email:      "test.user@example.com",
			Username:   "tuser",
			Role:       "student",
		},
	})

	assert.Equal(t, 1, created, "expected one new user")
	assert.Empty(t, errs, "expected no errors")
	assert.NotNil(t, captured, "expected Create to be called with a user")
	assert.NotEmpty(t, captured.PasswordHash, "expected initial password hash to be set")

	// The bug: previously the hash was bcrypt("OneRoster-"+sourcedID+"-changeme").
	// Verify the new hash REJECTS that string.
	leakedGuess := "OneRoster-" + sourcedID + "-changeme"
	err := bcrypt.CompareHashAndPassword([]byte(captured.PasswordHash), []byte(leakedGuess))
	assert.Error(t, err, "stored hash must NOT match the deterministic legacy string")
	assert.ErrorIs(t, err, bcrypt.ErrMismatchedHashAndPassword)

	// Wave 1.6 follow-up: the random password is irrecoverable, so
	// the OneRoster path MUST set RequiresPasswordReset so the
	// LoginPipeline gates session minting and forces the user to
	// choose a real password before getting a session.
	assert.True(t, captured.RequiresPasswordReset, "OneRoster-provisioned user must have RequiresPasswordReset=true")

	mockUserRepo.AssertExpectations(t)
}
