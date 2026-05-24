package handlers

// Tests for the inline-MIME allow-list (F-040). The pre-fix download
// path served any `image/*` content type inline, including
// `image/svg+xml`, which lets an attacker upload an SVG payload (or
// any file with the SVG MIME header — attacker-controlled in the
// multipart upload) and execute script in the LMS origin.
//
// This is a same-package _test.go so it can call the unexported
// isInlineSafeMIME helper directly.

import "testing"

func TestIsInlineSafeMIME(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		want        bool
	}{
		// Allow-list — these render but cannot execute.
		{"png exact", "image/png", true},
		{"png with charset", "image/png; charset=binary", true},
		{"jpeg exact", "image/jpeg", true},
		{"gif exact", "image/gif", true},
		{"webp exact", "image/webp", true},
		{"avif exact", "image/avif", true},
		{"heic exact", "image/heic", true},

		// SVG — the CANONICAL XSS vehicle. MUST NOT be inline.
		{"svg blocked", "image/svg+xml", false},
		{"svg with charset", "image/svg+xml; charset=utf-8", false},

		// HTML / XML — script execution.
		{"html blocked", "text/html", false},
		{"xhtml blocked", "application/xhtml+xml", false},
		{"xml blocked", "application/xml", false},

		// PDF — embedded scripts in some viewers.
		{"pdf blocked", "application/pdf", false},

		// Common attacker payloads that try to look like images.
		{"text/plain blocked", "text/plain", false},
		{"javascript blocked", "application/javascript", false},
		{"octet-stream blocked", "application/octet-stream", false},

		// Edge cases.
		{"empty blocked", "", false},
		{"image-prefix-only blocked", "image/", false}, // not an exact match
		{"random gibberish blocked", "totally-fake/mime", false},
		{"case-insensitive png", "IMAGE/PNG", true},
		{"whitespace tolerated", "  image/png  ", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isInlineSafeMIME(tc.contentType)
			if got != tc.want {
				t.Fatalf("isInlineSafeMIME(%q) = %v, want %v", tc.contentType, got, tc.want)
			}
		})
	}
}
