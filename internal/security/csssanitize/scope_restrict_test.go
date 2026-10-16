package csssanitize

import (
	"strings"
	"testing"
)

// Selectors that would let a page stylesheet query the rest of Paper's
// document from inside the scoped page are dropped (audit repros first).
func TestScope_DropsSelectorsThatQueryTheDocument(t *testing.T) {
	const root = "#paper-page-5"
	dropped := []string{
		// The audit repros.
		`html:has(input[name="q"][value^="se"]) p{color:red}`,
		`p:is(html:has(a[href*="/users/77"]) *){color:red}`,
		// Qualifiers on html / body / :root: classes and [attr] / [attr=v] only.
		`html:not(.x) p{color:red}`,
		`body:has(.x) p{color:red}`,
		`:root:has(~ div){color:red}`,
		`:root:has(+ div) p{color:red}`,
		`html[lang^="e"] p{color:red}`,
		`html[lang*="e"]{color:red}`,
		`html body:hover p{color:red}`,
		`html:lang(en) p{color:red}`,
		`html#doc p{color:red}`,
		// Combinators inside :is() / :where() / :not().
		`p:is(.a .b){color:red}`,
		`p:where(.a > .b){color:red}`,
		`p:not(.a ~ .b){color:red}`,
		`p:not(.a + .b){color:red}`,
		`p:is(:has(a) *){color:red}`,
		// html, body, :root, :scope inside any selector function.
		`p:is(html){color:red}`,
		`p:where(body *){color:red}`,
		`p:not(:root){color:red}`,
		`p:is(:scope *){color:red}`,
		`p:has(html){color:red}`,
		`:is(:root) p{color:red}`,
		// A selector list inside :nth-child() is a selector too.
		`li:nth-child(2 of .x){color:red}`,
		`li:nth-last-child(1 of html *){color:red}`,
	}
	for _, in := range dropped {
		if got := Scope(in, root); got != "" {
			t.Errorf("Scope(%s) = %s, want it dropped", in, got)
		}
	}

	kept := map[string]string{
		`html.dark p{color:red}`:                       `html.dark #paper-page-5 p{color:red}`,
		`html[data-theme] body.rtl p{color:red}`:       `html[data-theme] body.rtl #paper-page-5 p{color:red}`,
		`:root[lang="es"] q{quotes:none}`:              `html[lang="es"] #paper-page-5 q{quotes:none}`,
		`:root[lang=es]{color:red}`:                    `html[lang=es] #paper-page-5{color:red}`,
		`:is(h1, h2) a{color:red}`:                     `#paper-page-5 :is(h1, h2) a{color:red}`,
		`p:is( .a , .b ){color:red}`:                   `#paper-page-5 p:is(.a, .b){color:red}`,
		`p:not(.a, .b){color:red}`:                     `#paper-page-5 p:not(.a, .b){color:red}`,
		`li:has(> img, + p){color:red}`:                `#paper-page-5 li:has(> img, + p){color:red}`,
		`p:not(:has(> a)){color:red}`:                  `#paper-page-5 p:not(:has(> a)){color:red}`,
		`li:nth-child(2n + 1){color:red}`:              `#paper-page-5 li:nth-child(2n + 1){color:red}`,
		`a:is([rel=html], [data-x="body"]){color:red}`: `#paper-page-5 a:is([rel=html], [data-x="body"]){color:red}`,
		`.card:has(img) h2{color:red}`:                 `#paper-page-5 .card:has(img) h2{color:red}`,
	}
	for in, want := range kept {
		got := Scope(in, root)
		if got != want {
			t.Errorf("Scope(%s)\n got %s\nwant %s", in, got, want)
		}
		checkScoped(t, got, root)
	}

	// The rest of a rule's selector list survives one dropped selector.
	if got := Scope(`p:is(html *), .ok{color:red}`, root); got != `#paper-page-5 .ok{color:red}` {
		t.Errorf("selector list: %s", got)
	}
}

// A sheet within MaxBytes grows when Sanitize reformats it; Scope must still
// scope it rather than silently returning nothing.
func TestScope_AcceptsSanitizedSheetOfMaxSize(t *testing.T) {
	var b strings.Builder
	for i := 0; b.Len() < MaxBytes-64; i++ {
		b.WriteString(".c")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString("{color:red;margin:0}")
	}
	in := b.String()
	out, err := Sanitize(in)
	if err != nil {
		t.Fatalf("Sanitize: %v", err)
	}
	if again, err := Resanitize(out); err != nil || again != out {
		t.Fatalf("Resanitize is not idempotent on a large sanitized sheet (err %v)", err)
	}
	if len(out) <= MaxBytes {
		t.Fatalf("test sheet should grow past MaxBytes when sanitized (%d bytes)", len(out))
	}
	scoped := Scope(out, "#paper-page-9")
	if scoped == "" || !strings.HasPrefix(scoped, "#paper-page-9 .c{color:red;margin:0}") {
		t.Fatalf("a valid sheet lost its CSS: %.80q", scoped)
	}
	if Scope(strings.Repeat(" ", MaxScopeBytes+1), "#r") != "" {
		t.Error("input over MaxScopeBytes accepted")
	}
	if _, err := Resanitize(strings.Repeat(" ", MaxScopeBytes+1)); err == nil {
		t.Error("Resanitize accepted input over MaxScopeBytes")
	}
	// A sheet that would sanitize to more than MaxScopeBytes is refused up
	// front rather than stored and then scoped to nothing.
	if _, err := Sanitize(strings.Repeat("a{b:c}", MaxBytes/6)); err == nil {
		t.Error("Sanitize returned a sheet larger than MaxScopeBytes")
	}
}
