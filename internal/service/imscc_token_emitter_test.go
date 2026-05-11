package service

import (
	"strings"
	"testing"
)

func TestEmitTokens_RoundTripsFromImportRewrite(t *testing.T) {
	ctx := &tokenEmitterCtx{
		courseID:            7,
		filePathByID:        map[uint]string{42: "web_resources/diagrams/cell.png"},
		pageSlugs:           map[string]bool{"intro-to-fractions": true},
		migrationIDByEntity: map[string]string{"Assignment:9": "g_asg_orig", "Quiz:11": "g_quiz_orig"},
	}
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "file download → IMS-CC-FILEBASE",
			in:   `<img src="/api/v1/files/42/download" alt="x">`,
			want: `<img src="$IMS-CC-FILEBASE$/diagrams%2Fcell.png" alt="x">`,
		},
		{
			name: "page link → WIKI_REFERENCE",
			in:   `<a href="/courses/7/pages/intro-to-fractions">go</a>`,
			want: `<a href="$WIKI_REFERENCE$/wiki/intro-to-fractions">go</a>`,
		},
		{
			name: "assignment link → CANVAS_OBJECT_REFERENCE",
			in:   `See <a href="/courses/7/assignments/9">homework</a>.`,
			want: `See <a href="$CANVAS_OBJECT_REFERENCE$/assignments/g_asg_orig">homework</a>.`,
		},
		{
			name: "quiz link with synthesized migration id",
			in:   `<a href="/courses/7/quizzes/11">quiz</a>`,
			want: `<a href="$CANVAS_OBJECT_REFERENCE$/quizzes/g_quiz_orig">quiz</a>`,
		},
		{
			name: "quiz link with default migration id",
			in:   `<a href="/courses/7/quizzes/99">other</a>`,
			want: `<a href="$CANVAS_OBJECT_REFERENCE$/quizzes/g99">other</a>`,
		},
		{
			name: "unknown file id stays absolute",
			in:   `<img src="/api/v1/files/9999/download">`,
			want: `<img src="/api/v1/files/9999/download">`,
		},
		{
			name: "cross-course link is left alone",
			in:   `<a href="/courses/3/pages/intro-to-fractions">other course</a>`,
			want: `<a href="/courses/3/pages/intro-to-fractions">other course</a>`,
		},
		{
			name: "unknown page slug stays absolute",
			in:   `<a href="/courses/7/pages/no-such-page">stale link</a>`,
			want: `<a href="/courses/7/pages/no-such-page">stale link</a>`,
		},
		{
			name: "empty body is empty",
			in:   ``,
			want: ``,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := emitTokens(c.in, ctx)
			if got != c.want {
				t.Errorf("got %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestEmitTokens_ImportRewriteIsIdempotentRoundTrip(t *testing.T) {
	// Take an HTML body that has been through the IMPORT rewriter (Canvas
	// tokens → internal URLs), run it through the EXPORT emitter, and
	// verify the original Canvas tokens come back. This is the critical
	// round-trip property the test cartridge depends on.
	imported := `<p>See <a href="/courses/7/pages/intro-to-fractions">the intro</a> ` +
		`and <img src="/api/v1/files/42/download"> for context.</p>`
	ctx := &tokenEmitterCtx{
		courseID:     7,
		filePathByID: map[uint]string{42: "web_resources/diagrams/cell.png"},
		pageSlugs:    map[string]bool{"intro-to-fractions": true},
	}
	out := emitTokens(imported, ctx)
	if !strings.Contains(out, "$WIKI_REFERENCE$/wiki/intro-to-fractions") {
		t.Errorf("WIKI_REFERENCE missing: %s", out)
	}
	if !strings.Contains(out, "$IMS-CC-FILEBASE$/") {
		t.Errorf("IMS-CC-FILEBASE missing: %s", out)
	}
	// And the Canvas-style tokens from the output match what the importer's
	// regex would consume (basic sanity check that we didn't double-escape).
	if strings.Contains(out, "/api/v1/files/") {
		t.Errorf("internal file URL still present: %s", out)
	}
}
