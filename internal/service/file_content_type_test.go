package service

import "testing"

func TestServedAndRecordedContentType(t *testing.T) {
	scripts := []string{
		"text/javascript", "application/javascript", "application/x-javascript", "text/ecmascript",
		"application/ecmascript", "text/jscript", "TEXT/JavaScript; charset=utf-8", " text/x-javascript ",
		"application/x-ecmascript", "text/javascript1.5", "text/livescript",
	}
	for _, ct := range scripts {
		if got := ServedContentType(ct); got != OctetStream {
			t.Errorf("ServedContentType(%q) = %q", ct, got)
		}
		if got := RecordedContentType(ct); got != OctetStream {
			t.Errorf("RecordedContentType(%q) = %q", ct, got)
		}
	}
	for ct, want := range map[string]string{
		"":                          OctetStream,
		"text/css":                  OctetStream,
		"text/css; charset=utf-8":   OctetStream,
		"image/png":                 "image/png",
		"application/pdf":           "application/pdf",
		"text/plain; charset=utf-8": "text/plain; charset=utf-8",
		// Already attachments with nosniff; SVG keeps its type for <img>.
		"text/html":     "text/html",
		"image/svg+xml": "image/svg+xml",
		"video/mp4":     "video/mp4",
	} {
		if got := ServedContentType(ct); got != want {
			t.Errorf("ServedContentType(%q) = %q, want %q", ct, got, want)
		}
	}
	if got := RecordedContentType("text/css"); got != "text/css" {
		t.Errorf("RecordedContentType(text/css) = %q: only script types change on upload", got)
	}
}
