package csssanitize

import (
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// This file turns a stylesheet into rules the way a browser does: the
// tdewolff CSS Syntax Level 3 tokenizer, then the "consume a list of rules" /
// "consume a block" algorithms from the same spec. Block structure is
// decided by bracket matching over the whole token stream exactly as the
// spec does it (a "}" inside an unclosed "(" does not end a rule), so what
// this parser calls a rule is what the browser will call a rule.

// token is one CSS token. Comments are turned into whitespace and runs of
// whitespace are merged before parsing.
type token struct {
	tt   css.TokenType
	data string
}

func (t token) is(tt css.TokenType) bool { return t.tt == tt }

func (t token) delim(c byte) bool {
	return t.tt == css.DelimToken && len(t.data) == 1 && t.data[0] == c
}

func (t token) ident(name string) bool {
	return t.tt == css.IdentToken && strings.EqualFold(t.data, name)
}

// tokenize lexes s. Invalid UTF-8 becomes U+FFFD and control characters
// other than whitespace are removed first, so the lexer never sees a NUL.
func tokenize(s string) []token {
	s = strings.ToValidUTF8(s, "�")
	s = strings.Map(func(r rune) rune {
		if (r < 0x20 && r != '\t' && r != '\n' && r != '\r' && r != '\f') || r == 0x7f {
			return -1
		}
		return r
	}, s)
	l := css.NewLexer(parse.NewInputString(s))
	var toks []token
	for {
		tt, data := l.Next()
		if tt == css.ErrorToken {
			break
		}
		if tt == css.CommentToken {
			tt, data = css.WhitespaceToken, []byte(" ")
		}
		if tt == css.WhitespaceToken {
			if n := len(toks); n > 0 && toks[n-1].tt == css.WhitespaceToken {
				continue
			}
			data = []byte(" ")
		}
		toks = append(toks, token{tt: tt, data: string(data)})
	}
	return toks
}

// parser holds a token stream and, for every opening token ("(", "[", "{"
// or a function), the index of the token that closes it (len(toks) when the
// stylesheet ends first).
type parser struct {
	toks  []token
	match []int
}

func newParser(s string) *parser {
	toks := tokenize(s)
	match := make([]int, len(toks))
	var stack []int
	for i, t := range toks {
		switch t.tt {
		case css.LeftParenthesisToken, css.FunctionToken, css.LeftBracketToken, css.LeftBraceToken:
			stack = append(stack, i)
		case css.RightParenthesisToken, css.RightBracketToken, css.RightBraceToken:
			// A closer only closes the innermost open block of its own kind;
			// anything else is an ordinary token inside that block.
			if n := len(stack); n > 0 && closerOf(toks[stack[n-1]].tt) == t.tt {
				match[stack[n-1]] = i
				stack = stack[:n-1]
			}
		}
	}
	for _, i := range stack {
		match[i] = len(toks)
	}
	return &parser{toks: toks, match: match}
}

func closerOf(tt css.TokenType) css.TokenType {
	switch tt {
	case css.LeftParenthesisToken, css.FunctionToken:
		return css.RightParenthesisToken
	case css.LeftBracketToken:
		return css.RightBracketToken
	case css.LeftBraceToken:
		return css.RightBraceToken
	}
	return css.ErrorToken
}

func isOpener(tt css.TokenType) bool { return closerOf(tt) != css.ErrorToken }

// skip returns the index just after the component value starting at i: a
// whole block for an opening token, otherwise the token itself.
func (p *parser) skip(i int) int {
	if isOpener(p.toks[i].tt) {
		return p.match[i] + 1
	}
	return i + 1
}

// span is a half-open range of token indices.
type span struct{ lo, hi int }

// rawRule is a rule as the browser sees it, before any policy is applied.
type rawRule struct {
	at       string // lowercased at-rule name without "@"; "" for a style rule
	prelude  span   // selector list, or the at-rule prelude
	block    span   // contents of the {} block
	hasBlock bool
}

// rules consumes a list of rules (top level or the body of a block at-rule)
// in [lo, hi).
func (p *parser) rules(lo, hi int) []rawRule {
	var out []rawRule
	i := lo
	for i < hi {
		t := p.toks[i]
		switch t.tt {
		case css.WhitespaceToken, css.CDOToken, css.CDCToken, css.SemicolonToken, css.RightBraceToken:
			i++
			continue
		}
		var r rawRule
		if t.tt == css.AtKeywordToken {
			r, i = p.atRule(i, hi)
		} else {
			var ok bool
			r, ok, i = p.qualifiedRule(i, hi)
			if !ok {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// atRule consumes an at-rule starting at i: a prelude ending in ";" or a
// {} block (or the end of the range).
func (p *parser) atRule(i, hi int) (rawRule, int) {
	r := rawRule{at: strings.ToLower(strings.TrimPrefix(p.toks[i].data, "@"))}
	j := i + 1
	for j < hi {
		switch p.toks[j].tt {
		case css.SemicolonToken:
			r.prelude = span{i + 1, j}
			return r, j + 1
		case css.LeftBraceToken:
			end := min(p.match[j], hi)
			r.prelude = span{i + 1, j}
			r.block = span{j + 1, end}
			r.hasBlock = true
			return r, end + 1
		}
		j = min(p.skip(j), hi)
	}
	r.prelude = span{i + 1, hi}
	return r, hi
}

// qualifiedRule consumes a style rule starting at i. A prelude that runs to
// the end of the range without a block is a parse error: ok is false.
func (p *parser) qualifiedRule(i, hi int) (rawRule, bool, int) {
	j := i
	for j < hi {
		if p.toks[j].tt == css.LeftBraceToken {
			end := min(p.match[j], hi)
			return rawRule{prelude: span{i, j}, block: span{j + 1, end}, hasBlock: true}, true, end + 1
		}
		j = min(p.skip(j), hi)
	}
	return rawRule{}, false, hi
}

// rawDecl is one declaration: its name token and value tokens (after ":").
type rawDecl struct {
	name  token
	value span
}

// declarations consumes the contents of a style rule's block. Nested
// at-rules and nested style rules (CSS nesting) are skipped: they are not
// supported, so they are dropped rather than scoped wrongly.
func (p *parser) declarations(lo, hi int) []rawDecl {
	var out []rawDecl
	i := lo
	for i < hi {
		t := p.toks[i]
		if t.tt == css.WhitespaceToken || t.tt == css.SemicolonToken {
			i++
			continue
		}
		// Find the end of this item: a top-level ";" or, for a nested rule,
		// the end of its {} block.
		j := i
		nested := false
		for j < hi && p.toks[j].tt != css.SemicolonToken {
			if p.toks[j].tt == css.LeftBraceToken {
				nested = true
				j = min(p.match[j], hi) + 1
				break
			}
			j = min(p.skip(j), hi)
		}
		start := i
		i = j
		if nested || t.tt == css.AtKeywordToken {
			continue
		}
		if t.tt != css.IdentToken && t.tt != css.CustomPropertyNameToken {
			continue
		}
		k := start + 1
		for k < j && p.toks[k].tt == css.WhitespaceToken {
			k++
		}
		if k >= j || p.toks[k].tt != css.ColonToken {
			continue
		}
		end := min(j, hi)
		out = append(out, rawDecl{name: t, value: span{k + 1, end}})
	}
	return out
}

// tokensIn returns the tokens of s with leading and trailing whitespace
// removed.
func (p *parser) tokensIn(s span) []token {
	toks := p.toks[s.lo:s.hi]
	for len(toks) > 0 && toks[0].tt == css.WhitespaceToken {
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].tt == css.WhitespaceToken {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// joinTokens writes tokens back out with each whitespace run as one space
// and no leading or trailing whitespace.
func joinTokens(toks []token) string {
	var b strings.Builder
	for i, t := range toks {
		if t.tt == css.WhitespaceToken {
			if i == 0 || i == len(toks)-1 {
				continue
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteString(t.data)
	}
	return b.String()
}

// splitTopLevel splits toks at commas that are not inside a block.
func splitTopLevel(toks []token) [][]token {
	var parts [][]token
	depth := 0
	start := 0
	for i, t := range toks {
		switch {
		case isOpener(t.tt):
			depth++
		case t.tt == css.RightParenthesisToken || t.tt == css.RightBracketToken || t.tt == css.RightBraceToken:
			if depth > 0 {
				depth--
			}
		case t.tt == css.CommaToken && depth == 0:
			parts = append(parts, toks[start:i])
			start = i + 1
		}
	}
	return append(parts, toks[start:])
}

// trimWS removes leading and trailing whitespace tokens.
func trimWS(toks []token) []token {
	for len(toks) > 0 && toks[0].tt == css.WhitespaceToken {
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].tt == css.WhitespaceToken {
		toks = toks[:len(toks)-1]
	}
	return toks
}
