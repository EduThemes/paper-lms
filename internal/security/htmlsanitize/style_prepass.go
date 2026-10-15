package htmlsanitize

import (
	"strings"

	"github.com/aymerick/douceur/parser"
	"golang.org/x/net/html"
)

// stripEscapedStyleDeclarations removes every inline-style declaration that
// contains a backslash before the HTML reaches bluemonday.
//
// bluemonday hands a style value to its handler after decoding CSS hex
// escapes (and lowercasing), but writes the original, still-escaped value to
// the output. A validator therefore never sees exactly what the browser will
// parse. No editor or designer output needs escapes in inline CSS, so any
// declaration with a backslash in its property or value is dropped here and
// the validator only ever judges the bytes that are written out.
//
// The rest of the document is copied through byte for byte: only start tags
// whose style attribute contains a backslash are re-serialized. bluemonday
// tokenizes with the same golang.org/x/net/html tokenizer, so it sees the
// same tags and attributes.
func stripEscapedStyleDeclarations(input string) string {
	// Fast path: the tokenizer decodes character references in attribute
	// values, so "&#92;" is a backslash too.
	if !strings.Contains(input, `\`) && !strings.Contains(input, "&") {
		return input
	}
	z := html.NewTokenizer(strings.NewReader(input))
	var b strings.Builder
	b.Grow(len(input))
	changed := false
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			// io.EOF, or a tokenizer error: either way, what has been read is
			// in b and anything unread is unparseable for bluemonday too.
			break
		}
		raw := string(z.Raw())
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			b.WriteString(raw)
			continue
		}
		tok := z.Token()
		rewrote := false
		attrs := tok.Attr[:0]
		for _, a := range tok.Attr {
			if a.Namespace == "" && strings.EqualFold(a.Key, "style") && strings.Contains(a.Val, `\`) {
				rewrote = true
				a.Val = dropEscapedDeclarations(a.Val)
				if a.Val == "" {
					continue
				}
			}
			attrs = append(attrs, a)
		}
		if !rewrote {
			b.WriteString(raw)
			continue
		}
		changed = true
		tok.Attr = attrs
		b.WriteString(tok.String())
	}
	if !changed {
		return input
	}
	return b.String()
}

// dropEscapedDeclarations parses a style attribute with the same parser
// bluemonday uses and keeps the declarations without a backslash. A value
// that does not parse is dropped whole (bluemonday would drop it too).
func dropEscapedDeclarations(style string) string {
	style = strings.TrimRight(style, " ")
	if style != "" && !strings.HasSuffix(style, ";") {
		style += ";"
	}
	decs, err := parser.ParseDeclarations(style)
	if err != nil {
		return ""
	}
	kept := make([]string, 0, len(decs))
	for _, d := range decs {
		if strings.Contains(d.Property, `\`) || strings.Contains(d.Value, `\`) {
			continue
		}
		kept = append(kept, d.Property+": "+d.Value)
	}
	return strings.Join(kept, "; ")
}
