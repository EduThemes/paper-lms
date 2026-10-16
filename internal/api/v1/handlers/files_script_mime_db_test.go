package handlers_test

// An upload that claims a script type must never be served with it: under
// "script-src 'self'" a <script src="/api/v1/files/N/download"> would run.
// All data is made up.

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestDownloadFile_ScriptTypesServedAsOctetStream(t *testing.T) {
	f := newFilesLaneFixture(t)
	teacher := f.app(flTeacher, "TeacherEnrollment")

	// Unknown extension (no MIME check) with a script Content-Type.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="x.txtx"`)
	hdr.Set("Content-Type", "text/javascript")
	fw, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = fw.Write([]byte("alert(document.domain)"))
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/courses/%d/files", flCourse), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := teacher.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	created, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", created["content_type"], "the script type is not recorded")

	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/files/%v/download", created["id"]), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))

	// A row stored before the fix still carries the script type: the
	// download maps it.
	_, legacy := flUpload(t, teacher, flCourse, "old.txtx", "alert(1)", nil)
	require.NoError(t, f.g.Model(&models.Attachment{}).Where("id = ?", legacy["id"]).
		Update("content_type", "application/x-javascript; charset=utf-8").Error)
	resp = testutil.MakeRequest(teacher, http.MethodGet, fmt.Sprintf("/files/%v/download", legacy["id"]), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
}
