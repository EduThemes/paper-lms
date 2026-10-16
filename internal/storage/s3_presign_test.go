package storage

import (
	"context"
	"net/url"
	"testing"
)

// A presigned download can carry the Content-Type S3 must answer with, so
// a redirect download gets the same safe type as a proxied one. Presigning
// is local: no request reaches the (made-up) endpoint.
func TestS3URLWithContentType(t *testing.T) {
	ctx := context.Background()
	b, err := NewS3Backend(ctx, S3Config{
		Bucket: "paper-test", Region: "us-east-1", Endpoint: "http://127.0.0.1:9",
		AccessKey: "AKIDEXAMPLE", SecretKey: "not-a-real-secret",
	}, nil)
	if err != nil {
		t.Fatalf("NewS3Backend: %v", err)
	}
	var _ ContentTypeURLer = b

	raw, err := b.URLWithContentType(ctx, "Course/1/x/payload.txtx", "application/octet-stream")
	if err != nil {
		t.Fatalf("URLWithContentType: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	if got := u.Query().Get("response-content-type"); got != "application/octet-stream" {
		t.Errorf("response-content-type = %q in %s", got, raw)
	}

	plain, err := b.URL(ctx, "Course/1/x/payload.txtx")
	if err != nil {
		t.Fatalf("URL: %v", err)
	}
	if pu, _ := url.Parse(plain); pu.Query().Has("response-content-type") {
		t.Errorf("URL should not override the type: %s", plain)
	}
}
