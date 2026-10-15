package designdoc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// engineDoc is a design shaped like the engine's output (made-up content).
const engineDoc = `{
  "version": 1, "id": "doc-1", "themeId": "tide-pool", "paletteId": "default", "layout": "classic-band",
  "kind": "home", "locale": "en",
  "palette": {"id": "custom-1", "name": "Ocean", "primary": "#12355B", "onPrimary": "#ffffff",
    "secondary": "#2a6f97", "onSecondary": "#ffffff", "accent": "#f5c518", "surface": "#eef6fb",
    "onSurface": "#1f2328", "background": "#ffffff", "text": "#1f2328", "link": "#0b5cad"},
  "options": {"density": "comfortable", "headingStyle": "band"},
  "customCss": "/* mine */\np { color: red; }\n",
  "sections": [
    {"id": "s1", "type": "title", "text": "Biology 101", "subtitle": "Fall"},
    {"id": "s2", "type": "banner", "imageId": "banner", "alt": ""},
    {"id": "s3", "type": "welcome", "heading": "Welcome", "body": "<p>Don't forget &amp; enjoy&nbsp;it.<br>Line two with <a href=\"/courses/{course}/modules\">modules</a>.</p><ul><li><strong>Bold</strong> <em>it</em></li></ul>"},
    {"id": "s4", "type": "quick-links", "heading": "Start", "links": [
      {"id": "l1", "label": "Syllabus", "href": "/courses/{course}/syllabus", "icon": "file-text", "slot": "syllabus"},
      {"id": "l2", "label": "Library", "href": "https://library.example.edu/?q=a&b=c", "icon": "library", "description": "Find books"},
      {"id": "l3", "label": "Mail", "href": "mailto:jane@example.test", "icon": "mail", "moduleId": "12"}
    ]},
    {"id": "s5", "type": "instructor", "heading": "Your instructor", "name": "Jane Doe", "role": "Teacher",
      "email": "jane@example.test", "officeHours": "Mon 2-3pm", "photoSrc": "/api/v1/files/42/download", "photoAlt": "Jane", "bio": "<p>Hi!</p>"},
    {"id": "s6", "type": "callout", "variant": "tip", "heading": "Tip", "body": "<p>Read first.</p>"},
    {"id": "s7", "type": "objectives", "heading": "Goals", "items": ["Learn cells", "Use a microscope"]},
    {"id": "s8", "type": "schedule", "heading": "Weeks", "items": [{"id": "w1", "title": "Week 1", "dates": "Sep 1", "summary": "Intro", "href": "/courses/{course}/modules/3"}]},
    {"id": "s9", "type": "this-week", "heading": "This week", "title": "Week 3", "dates": "Sep 15", "moduleId": "7",
      "items": [{"id": "t1", "label": "Quiz", "href": "/courses/{course}/quizzes/1", "due": "Fri"}]},
    {"id": "s10", "type": "text", "heading": "More", "body": "<ol><li>One</li></ol>"},
    {"id": "s11", "type": "divider"},
    {"id": "s12", "type": "banner", "customSrc": "/design-assets/themes/tide-pool/banner@2x.webp", "alt": "Shore"}
  ]
}`

func decodeAny(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("invalid JSON %s: %v", b, err)
	}
	return v
}

func validate(t *testing.T, doc string) map[string]any {
	t.Helper()
	out, err := ValidateDesign([]byte(doc))
	if err != nil {
		t.Fatalf("ValidateDesign: %v", err)
	}
	again, err := ValidateDesign(out)
	if err != nil || string(again) != string(out) {
		t.Fatalf("ValidateDesign is not idempotent:\n%s\n%s", out, again)
	}
	return decodeAny(t, out).(map[string]any)
}

func section(t *testing.T, doc map[string]any, i int) map[string]any {
	t.Helper()
	secs := doc["sections"].([]any)
	if i >= len(secs) {
		t.Fatalf("no section %d in %v", i, secs)
	}
	return secs[i].(map[string]any)
}

// A design the engine produced comes back with exactly the same values, so
// the page's data-pd-hash keeps matching it.
func TestValidateDesign_EngineDocUnchanged(t *testing.T) {
	got := validate(t, engineDoc)
	want := decodeAny(t, []byte(engineDoc))
	if !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", " ")
		t.Errorf("engine doc changed:\n%s", g)
	}
}

// The audit repro: markup and addresses the designer would write into the
// page are cleaned on the server.
func TestValidateDesign_AuditRepro(t *testing.T) {
	doc := `{"version":1,"id":"x","sections":[
	  {"id":"a","type":"welcome","heading":"Hi","body":"<p>Hello</p><img src=x onerror=alert(1)><iframe srcdoc=\"<script>alert(1)</script>\"></iframe>"},
	  {"id":"b","type":"quick-links","heading":"Links","links":[
	    {"id":"l1","label":"Bad","href":"javascript:alert(1)","icon":"x"},
	    {"id":"l2","label":"Encoded","href":"&#106;avascript:alert(1)","icon":"x"},
	    {"id":"l3","label":"Tab","href":"java\tscript:alert(1)","icon":"x"},
	    {"id":"l4","label":"Good","href":"/courses/{course}/grades","icon":"x"}]},
	  {"id":"c","type":"banner","customSrc":"data:image/svg+xml,<svg onload=alert(1)>","alt":""},
	  {"id":"d","type":"instructor","heading":"Me","name":"Jane Doe","photoSrc":"javascript:alert(1)","bio":"<a href=\"javascript:alert(1)\">click</a><script>alert(1)</script>"},
	  {"id":"e","type":"schedule","heading":"S","items":[{"id":"i","title":"T","href":"vbscript:x"}]},
	  {"id":"f","type":"this-week","heading":"W","title":"T","items":[{"id":"i","label":"L","href":"data:text/html,x"}]}
	]}`
	got := validate(t, doc)
	raw, _ := json.Marshal(got)
	for _, bad := range []string{"<img", "onerror", "<iframe", "srcdoc", "<script", "javascript:", "vbscript:", "data:", "&#106;"} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("cleaned design still contains %q:\n%s", bad, raw)
		}
	}
	if body := section(t, got, 0)["body"]; body != "<p>Hello</p>" {
		t.Errorf("welcome body = %q", body)
	}
	links := section(t, got, 1)["links"].([]any)
	if len(links) != 4 {
		t.Fatalf("links = %v", links)
	}
	for i := 0; i < 3; i++ {
		if _, ok := links[i].(map[string]any)["href"]; ok {
			t.Errorf("unsafe href kept on link %d: %v", i, links[i])
		}
	}
	if links[3].(map[string]any)["href"] != "/courses/{course}/grades" {
		t.Errorf("safe href lost: %v", links[3])
	}
	if _, ok := section(t, got, 2)["customSrc"]; ok {
		t.Error("data: customSrc kept")
	}
	inst := section(t, got, 3)
	if _, ok := inst["photoSrc"]; ok {
		t.Error("javascript: photoSrc kept")
	}
	if inst["bio"] != "click" {
		t.Errorf("bio = %q", inst["bio"])
	}
}

func TestValidateDesign_SchemaAndLimits(t *testing.T) {
	var secs []string
	for i := 0; i < MaxDesignSections+5; i++ {
		secs = append(secs, fmt.Sprintf(`{"id":"s%d","type":"divider"}`, i))
	}
	got := validate(t, `{"version":1,"id":"x","sections":[`+strings.Join(secs, ",")+`]}`)
	if n := len(got["sections"].([]any)); n != MaxDesignSections {
		t.Errorf("sections kept = %d", n)
	}

	var items []string
	for i := 0; i < MaxDesignListItems+5; i++ {
		items = append(items, `"item"`)
	}
	long := strings.Repeat("é", 3000)
	doc := `{"version":1,"id":"x","script":"alert(1)","options":{"density":"compact","onload":"x"},
	  "palette":{"id":"p","name":"n","primary":"red"},
	  "sections":[
	    {"id":"a","type":"objectives","heading":"` + strings.Repeat("h", 300) + `","items":[` + strings.Join(items, ",") + `]},
	    {"id":"b","type":"marquee","text":"x"},
	    "not an object",
	    {"id":"c","type":"title","text":"` + long + `","subtitle":"bell\u0007 and\u0000 nul","onclick":"x","html":"<b>x</b>"},
	    {"id":"d","type":"text","body":7,"heading":["x"]},
	    {"id":"e","type":"quick-links","heading":"L","links":["x",{"id":"l","label":"ok","href":"//cdn.example.com/x"}]},
	    {"id":"f","type":"banner","customSrc":"http://example.com/a.png"},
	    {"id":"g","type":"banner","customSrc":"//example.com/a.png"},
	    {"id":"h","type":"banner","customSrc":"https://images.example.com/a.png?w=2"},
	    {"id":"i","type":"banner","customSrc":"/design-assets/../api/v1/users"},
	    {"id":"j","type":"banner","customSrc":"/api/v1/files/7/download?verifier=abc"},
	    {"id":"k","type":"banner","customSrc":"https://user:pw@example.com/a.png"}
	  ],
	  "customCss":"` + strings.Repeat("a", 210*1024) + `"}`
	got = validate(t, doc)
	if _, ok := got["script"]; ok {
		t.Error("unknown top-level member kept")
	}
	if _, ok := got["palette"]; ok {
		t.Error("invalid palette kept")
	}
	if !reflect.DeepEqual(got["options"], map[string]any{"density": "compact"}) {
		t.Errorf("options = %v", got["options"])
	}
	if n := len(got["customCss"].(string)); n > 200*1024 {
		t.Errorf("customCss is %d bytes", n)
	}
	secsOut := got["sections"].([]any)
	if len(secsOut) != 10 {
		t.Fatalf("want 10 sections (unknown type and non-object dropped), got %d: %v", len(secsOut), secsOut)
	}
	obj := section(t, got, 0)
	if n := len([]rune(obj["heading"].(string))); n != 200 {
		t.Errorf("heading runes = %d", n)
	}
	if n := len(obj["items"].([]any)); n != MaxDesignListItems {
		t.Errorf("objective items = %d", n)
	}
	title := section(t, got, 1)
	if n := len([]rune(title["text"].(string))); n != 2000 {
		t.Errorf("title text runes = %d", n)
	}
	if title["subtitle"] != "bell and nul" {
		t.Errorf("subtitle = %q", title["subtitle"])
	}
	if _, ok := title["onclick"]; ok {
		t.Error("unknown section member kept")
	}
	text := section(t, got, 2)
	if _, ok := text["body"]; ok {
		t.Error("non-string body kept")
	}
	if _, ok := text["heading"]; ok {
		t.Error("non-string heading kept")
	}
	ql := section(t, got, 3)["links"].([]any)
	if len(ql) != 1 || ql[0].(map[string]any)["href"] != "//cdn.example.com/x" {
		t.Errorf("links = %v", ql)
	}
	for i, want := range []bool{false, false, true, false, true, false} {
		_, kept := section(t, got, 4+i)["customSrc"]
		if kept != want {
			t.Errorf("banner %d customSrc kept = %v, want %v", i, kept, want)
		}
	}
}

func TestSafeHref(t *testing.T) {
	ok := []string{
		"/courses/{course}/modules", "https://example.com/a?b=c#d", "http://example.com", "mailto:a@example.test",
		"//cdn.example.com/x", "page-2", "?tab=1", "#top", "../x", "/a:b", "x/y:z", " https://example.com ",
	}
	bad := []string{
		"", "   ", "javascript:alert(1)", "JaVaScRiPt:alert(1)", " javascript:alert(1)", "java\tscript:alert(1)",
		"java\x00script:alert(1)", "&#106;avascript:alert(1)", "javascript&#58;alert(1)", "javascript&colon;alert(1)",
		"data:text/html,x", "vbscript:x", "tel:123", "ftp://example.com", "a:b", "foo:bar/baz",
	}
	for _, h := range ok {
		if !SafeHref(h) {
			t.Errorf("SafeHref(%q) = false", h)
		}
	}
	for _, h := range bad {
		if SafeHref(h) {
			t.Errorf("SafeHref(%q) = true", h)
		}
	}
}

func TestSanitizeRichText(t *testing.T) {
	cases := map[string]string{
		`<p>Don't &amp; <a href="/courses/{course}/x">x</a><br></p>`: `<p>Don't &amp; <a href="/courses/{course}/x">x</a><br></p>`,
		`<p>a<br/>b</p>`:                                                 `<p>a<br/>b</p>`,
		`<p onclick="x()">a</p>`:                                         `<p>a</p>`,
		`<a href="javascript:alert(1)">x</a>`:                            `x`,
		`<a href="JAVASCRIPT:alert(1)"><strong>x</strong></a>`:           `<strong>x</strong>`,
		`<a href="https://example.com/?a=1&amp;b=2">x</a>`:               `<a href="https://example.com/?a=1&amp;b=2">x</a>`,
		`<a href="/x" title="t">x</a>`:                                   `<a href="/x">x</a>`,
		`<a>bare</a>`:                                                    `bare`,
		`<div><h2>Head</h2><b>bold</b></div>`:                            `Headbold`,
		`<svg><script>alert(1)</script></svg><style>p{}</style><p>k</p>`: `<p>k</p>`,
		`<p>x</p><!-- c -->`:                                             `<p>x</p>`,
	}
	for in, want := range cases {
		if got := SanitizeRichText(in); got != want {
			t.Errorf("SanitizeRichText(%q)\n got %q\nwant %q", in, got, want)
		}
		if got := SanitizeRichText(want); got != want {
			t.Errorf("SanitizeRichText not stable on %q: %q", want, got)
		}
	}
}
