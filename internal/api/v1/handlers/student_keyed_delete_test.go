package handlers_test

// F-012 (student-keyed family): Delete handlers MUST refuse to delete
// rows owned by users in another tenant. The repo Delete signature
// takes (id, accountID) and applies a WHERE user_id IN (SELECT id
// FROM users WHERE account_id = ?). The defense-in-depth contract
// asserted here:
//
//   1) Cross-tenant delete returns 404 (per 13.1.E existence leak —
//      never 403, which would confirm the row exists in another
//      tenant).
//   2) The repo's Delete MUST be called with the CALLER's accountID
//      (not 0, not a hardcoded value). When the service-level guard
//      rejects the row first (e.g. comment_bank's userID mismatch),
//      Delete must not fire at all. When the service-level guard
//      passes but the row belongs to another tenant (the meaningful
//      F-012 case), the repo's WHERE is the last line of defense.
//
// The CommentBank family is exercised here as the representative
// student-keyed Delete handler. Notification and NotificationDelivery
// are repo-only widenings (no Delete handler exists on either today)
// so their signature contract is locked by `go build ./...` plus the
// updated MockNotificationRepository / MockCommentBankItemRepository
// mocks. The peer_review repo has no `Delete(ctx, id)` method (only
// `DeleteByAssignment` which was already widened in Wave B) — skipped
// intentionally.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// TestDeleteCommentBankItem_CrossTenantReturns404 — F-012 repro for
// the student-keyed family. A caller in tenant A tries to DELETE a
// comment_bank_item whose owning user lives in tenant B. The service
// layer's FindByID returns the row (no tenant scope at the comment_
// bank.FindByID layer today), then the service's userID mismatch
// branch fires → handler maps "unauthorized" to 404 per the 13.1.E
// existence-leak contract. Critically, the repo's Delete MUST NOT
// be called.
func TestDeleteCommentBankItem_CrossTenantReturns404(t *testing.T) {
	repo := new(mocks.MockCommentBankItemRepository)

	// Resource owned by a different user (in tenant B in the threat
	// model). Service-layer userID check rejects before Delete fires.
	repo.On("FindByID", mock.Anything, uint(42)).Return(&models.CommentBankItem{
		ID:     42,
		UserID: 99, // ← NOT the caller (userID=7)
	}, nil)

	svc := service.NewCommentBankService(repo)
	h := handlers.NewCommentBankHandler(svc)

	app := testutil.SetupTestApp()
	app.Delete("/comment_bank/:id",
		authStub(7, 1), // caller userID=7, tenant=1
		h.DeleteItem)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/comment_bank/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"cross-user DELETE must return 404 not 403 per 13.1.E")
	// The destructive Delete MUST NOT have fired — the service-layer
	// guard short-circuits before the repo call.
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteCommentBankItem_HandlerPassesCallerAccountID — F-012
// happy-path locks the contract that callerAccountID(c) reaches the
// repo. The service-layer userID guard passes (row's user matches
// the caller), so the repo's Delete fires; the test asserts it
// receives the caller's accountID, NOT 0 and NOT a hardcoded value.
// A regression here would silently restore the cross-tenant write
// vector — a row whose owning user_id happened to collide with the
// caller's user_id across tenants would delete unfiltered.
func TestDeleteCommentBankItem_HandlerPassesCallerAccountID(t *testing.T) {
	repo := new(mocks.MockCommentBankItemRepository)

	repo.On("FindByID", mock.Anything, uint(42)).Return(&models.CommentBankItem{
		ID:     42,
		UserID: 7, // matches the caller
	}, nil)
	// The load-bearing assertion: accountID arg MUST equal the
	// caller's tenant (1), not 0 and not a hardcoded constant.
	repo.On("Delete", mock.Anything, uint(42),
		mock.MatchedBy(func(acct uint) bool { return acct == 1 }),
	).Return(nil)

	svc := service.NewCommentBankService(repo)
	h := handlers.NewCommentBankHandler(svc)

	app := testutil.SetupTestApp()
	app.Delete("/comment_bank/:id",
		authStub(7, 1),
		h.DeleteItem)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/comment_bank/42", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	repo.AssertCalled(t, "Delete", mock.Anything, uint(42),
		mock.MatchedBy(func(acct uint) bool { return acct == 1 }))
}

// TestDeleteCommentBankItem_RepoDeleteCrossTenantNotFound — defense-
// in-depth proof. Simulates the corner case where a service-layer
// guard might be bypassed (e.g. a future refactor): if the repo's
// Delete is called with a cross-tenant accountID, it surfaces as
// gorm.ErrRecordNotFound (zero rows matched the WHERE), which the
// handler maps to a 400 today via BadRequest("record not found"). We
// assert on the not-OK status, not the specific code, because the
// failure mode is what matters — a row in another tenant is not
// deleted, and the handler does NOT 200-OK.
func TestDeleteCommentBankItem_RepoDeleteCrossTenantNotFound(t *testing.T) {
	repo := new(mocks.MockCommentBankItemRepository)

	repo.On("FindByID", mock.Anything, uint(42)).Return(&models.CommentBankItem{
		ID:     42,
		UserID: 7,
	}, nil)
	// Repo simulates the cross-tenant zero-rows-matched case by
	// returning gorm.ErrRecordNotFound. The service.Delete bubbles
	// the error; the handler does NOT 200-OK.
	repo.On("Delete", mock.Anything, uint(42), uint(1)).
		Return(errors.New(gorm.ErrRecordNotFound.Error()))

	svc := service.NewCommentBankService(repo)
	h := handlers.NewCommentBankHandler(svc)

	app := testutil.SetupTestApp()
	app.Delete("/comment_bank/:id",
		authStub(7, 1),
		h.DeleteItem)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/comment_bank/42", nil)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode,
		"repo Delete error MUST surface as non-200 — the cross-tenant row was not deleted")
}
