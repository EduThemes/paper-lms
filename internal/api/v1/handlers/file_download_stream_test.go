package handlers_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// Leftover (grading lane note): fasthttp streams a SendStream body after
// the handler returns and closes it itself. DownloadFile deferred
// reader.Close(), so the local-storage file was closed before a byte was
// sent and every download came back empty.
func TestDownloadFile_LocalStorageStreamsWholeFile(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("Lab notes line\n", 4096) // > fasthttp's small-body path
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(content), 0o600))

	attachments := new(mocks.MockAttachmentRepository)
	attachments.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Attachment{
		ID: 5, ContextType: "User", ContextID: 7, UserID: 7, Filename: "notes.txt", ContentType: "text/plain", StoragePath: "notes.txt",
	}, nil)
	svc := service.NewFileService(new(mocks.MockFolderRepository), attachments, dir)
	h := handlers.NewFileHandler(svc, new(mocks.MockEnrollmentRepository), nil, nil)

	app := testutil.SetupTestApp()
	app.Get("/files/:id/download", contentAuthStub(1, "", false), h.DownloadFile)
	resp := testutil.MakeRequest(app, http.MethodGet, "/files/5/download", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, len(content), len(body))
}
