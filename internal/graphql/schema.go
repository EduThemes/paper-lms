package graphql

import (
	"fmt"
	"strings"
	"unicode"
)

// Request represents an incoming GraphQL request.
type Request struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                `json:"operationName"`
}

// Response represents the outgoing GraphQL response.
type Response struct {
	Data   interface{}    `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a single error in a GraphQL response.
type GraphQLError struct {
	Message string   `json:"message"`
	Path    []string `json:"path,omitempty"`
}

// Field represents a parsed field selection in a GraphQL query.
type Field struct {
	Name      string
	Alias     string
	Arguments map[string]interface{}
	Fields    []Field
}

// parser holds state for the recursive-descent parser.
type parser struct {
	input []rune
	pos   int
}

// ParseQuery parses a simplified GraphQL query string and returns the selected
// fields. It supports: { fieldName(arg: "value", arg2: 123) { sub1 sub2 } }
// It does not handle fragments, directives, aliases with ":", mutations, or
// variable references ($var) in the initial version — but it does handle
// variable references when they appear as argument values.
func ParseQuery(query string) ([]Field, error) {
	p := &parser{input: []rune(query), pos: 0}
	p.skipWhitespace()

	// Skip optional operation type (query, mutation, subscription) and operation name
	if p.peekWord() == "query" || p.peekWord() == "mutation" || p.peekWord() == "subscription" {
		p.readWord() // consume the keyword
		p.skipWhitespace()
		// There might be an operation name
		if p.pos < len(p.input) && p.input[p.pos] != '{' && p.input[p.pos] != '(' {
			p.readWord() // consume the operation name
			p.skipWhitespace()
		}
		// There might be variable definitions (...) that we skip
		if p.pos < len(p.input) && p.input[p.pos] == '(' {
			p.skipParenthesized()
			p.skipWhitespace()
		}
	}

	if p.pos >= len(p.input) || p.input[p.pos] != '{' {
		return nil, fmt.Errorf("expected '{' at position %d", p.pos)
	}

	fields, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}

	return fields, nil
}

func (p *parser) parseSelectionSet() ([]Field, error) {
	if p.pos >= len(p.input) || p.input[p.pos] != '{' {
		return nil, fmt.Errorf("expected '{' at position %d", p.pos)
	}
	p.pos++ // consume '{'
	p.skipWhitespace()

	var fields []Field
	for p.pos < len(p.input) && p.input[p.pos] != '}' {
		field, err := p.parseField()
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
		p.skipWhitespace()
	}

	if p.pos >= len(p.input) || p.input[p.pos] != '}' {
		return nil, fmt.Errorf("expected '}' at position %d", p.pos)
	}
	p.pos++ // consume '}'
	p.skipWhitespace()

	return fields, nil
}

func (p *parser) parseField() (Field, error) {
	name := p.readWord()
	if name == "" {
		return Field{}, fmt.Errorf("expected field name at position %d", p.pos)
	}
	p.skipWhitespace()

	field := Field{
		Name:      name,
		Arguments: make(map[string]interface{}),
	}

	// Parse arguments if present
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		args, err := p.parseArguments()
		if err != nil {
			return Field{}, err
		}
		field.Arguments = args
		p.skipWhitespace()
	}

	// Parse sub-selection if present
	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		subFields, err := p.parseSelectionSet()
		if err != nil {
			return Field{}, err
		}
		field.Fields = subFields
	}

	return field, nil
}

func (p *parser) parseArguments() (map[string]interface{}, error) {
	if p.pos >= len(p.input) || p.input[p.pos] != '(' {
		return nil, fmt.Errorf("expected '(' at position %d", p.pos)
	}
	p.pos++ // consume '('
	p.skipWhitespace()

	args := make(map[string]interface{})

	for p.pos < len(p.input) && p.input[p.pos] != ')' {
		// Read argument name
		argName := p.readWord()
		if argName == "" {
			return nil, fmt.Errorf("expected argument name at position %d", p.pos)
		}
		p.skipWhitespace()

		// Expect ':'
		if p.pos >= len(p.input) || p.input[p.pos] != ':' {
			return nil, fmt.Errorf("expected ':' after argument name at position %d", p.pos)
		}
		p.pos++ // consume ':'
		p.skipWhitespace()

		// Read argument value
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		args[argName] = val
		p.skipWhitespace()

		// Skip optional comma
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
			p.skipWhitespace()
		}
	}

	if p.pos >= len(p.input) || p.input[p.pos] != ')' {
		return nil, fmt.Errorf("expected ')' at position %d", p.pos)
	}
	p.pos++ // consume ')'

	return args, nil
}

func (p *parser) parseValue() (interface{}, error) {
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of input at position %d", p.pos)
	}

	ch := p.input[p.pos]

	// String value: "..." or '...'
	if ch == '"' || ch == '\'' {
		return p.parseString(ch)
	}

	// Variable reference: $varName
	if ch == '$' {
		p.pos++ // consume '$'
		varName := p.readWord()
		if varName == "" {
			return nil, fmt.Errorf("expected variable name after '$' at position %d", p.pos)
		}
		return "$" + varName, nil
	}

	// Boolean or null
	word := p.peekWord()
	if word == "true" {
		p.readWord()
		return true, nil
	}
	if word == "false" {
		p.readWord()
		return false, nil
	}
	if word == "null" {
		p.readWord()
		return nil, nil
	}

	// Number (int or float)
	if ch == '-' || (ch >= '0' && ch <= '9') {
		return p.parseNumber()
	}

	// Enum value (unquoted identifier)
	if unicode.IsLetter(ch) || ch == '_' {
		return p.readWord(), nil
	}

	return nil, fmt.Errorf("unexpected character '%c' at position %d", ch, p.pos)
}

func (p *parser) parseString(quote rune) (string, error) {
	p.pos++ // consume opening quote
	var buf strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			escaped := p.input[p.pos]
			switch escaped {
			case 'n':
				buf.WriteRune('\n')
			case 't':
				buf.WriteRune('\t')
			case '\\':
				buf.WriteRune('\\')
			case '"':
				buf.WriteRune('"')
			case '\'':
				buf.WriteRune('\'')
			default:
				buf.WriteRune('\\')
				buf.WriteRune(escaped)
			}
			p.pos++
			continue
		}
		if ch == quote {
			p.pos++ // consume closing quote
			return buf.String(), nil
		}
		buf.WriteRune(ch)
		p.pos++
	}
	return "", fmt.Errorf("unterminated string starting at position %d", p.pos)
}

func (p *parser) parseNumber() (interface{}, error) {
	start := p.pos
	isFloat := false

	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
	}
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
	}
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		isFloat = true
		p.pos++
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}

	numStr := string(p.input[start:p.pos])

	if isFloat {
		var f float64
		_, err := fmt.Sscanf(numStr, "%f", &f)
		if err != nil {
			return nil, fmt.Errorf("invalid number '%s': %w", numStr, err)
		}
		return f, nil
	}

	var n int
	_, err := fmt.Sscanf(numStr, "%d", &n)
	if err != nil {
		return nil, fmt.Errorf("invalid integer '%s': %w", numStr, err)
	}
	return n, nil
}

func (p *parser) readWord() string {
	start := p.pos
	for p.pos < len(p.input) && (unicode.IsLetter(p.input[p.pos]) || unicode.IsDigit(p.input[p.pos]) || p.input[p.pos] == '_') {
		p.pos++
	}
	return string(p.input[start:p.pos])
}

func (p *parser) peekWord() string {
	saved := p.pos
	word := p.readWord()
	p.pos = saved
	return word
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && (unicode.IsSpace(p.input[p.pos]) || p.input[p.pos] == ',') {
		p.pos++
	}
	// Skip comments (# ...)
	if p.pos < len(p.input) && p.input[p.pos] == '#' {
		for p.pos < len(p.input) && p.input[p.pos] != '\n' {
			p.pos++
		}
		p.skipWhitespace()
	}
}

func (p *parser) skipParenthesized() {
	if p.pos >= len(p.input) || p.input[p.pos] != '(' {
		return
	}
	depth := 0
	for p.pos < len(p.input) {
		if p.input[p.pos] == '(' {
			depth++
		} else if p.input[p.pos] == ')' {
			depth--
			if depth == 0 {
				p.pos++
				return
			}
		}
		p.pos++
	}
}

// HasField returns true if the field list contains a field with the given name.
func HasField(fields []Field, name string) bool {
	for _, f := range fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

// GetField returns the field with the given name, or nil if not found.
func GetField(fields []Field, name string) *Field {
	for i := range fields {
		if fields[i].Name == name {
			return &fields[i]
		}
	}
	return nil
}
