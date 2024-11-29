package shcv

import "strings"

// parser represents a Helm template parser
type parser struct {
	input    string
	pos      int
	lineNum  int
	template string
}

// Token types for parsing
const (
	openBrace   = "{{"
	closeBrace  = "}}"
	valuePrefix = ".Values."
	defaultPipe = "|"
	defaultFunc = "default"
)

// ParseFile parses a template file and returns all value references
func ParseFile(content, templatePath string) []ValueRef {
	parser := newParser(content, templatePath)
	return parser.parse()
}

// newParser creates a new parser instance
func newParser(input, template string) *parser {
	return &parser{
		input:    input,
		pos:      0,
		lineNum:  1,
		template: template,
	}
}

// parse parses the entire input and returns all value references
func (p *parser) parse() []ValueRef {
	var refs []ValueRef
	for p.pos < len(p.input) {
		if p.match(openBrace) {
			if ref := p.parseValueRef(); ref != nil {
				refs = append(refs, *ref)
			}
		} else {
			if p.current() == '\n' {
				p.lineNum++
			}
			p.pos++
		}
	}
	return refs
}

// parseValueRef parses a single value reference
func (p *parser) parseValueRef() *ValueRef {
	start := p.pos

	// Skip whitespace after {{
	p.skipWhitespace()

	// Check for .Values. prefix
	if !p.match(valuePrefix) {
		p.pos = start + 2 // Skip {{ and continue
		return nil
	}

	// Parse the value path
	path := p.parseValuePath()
	if path == "" {
		return nil
	}

	// Look for default value
	var defaultValue string

	// Handle pipe operations
	for p.pos < len(p.input) {
		p.skipWhitespace()
		if !p.match(defaultPipe) {
			break
		}

		p.skipWhitespace()
		if p.match(defaultFunc) {
			p.skipWhitespace()
			defaultValue = p.parseDefaultValue()
		}
		// Skip other functions until next pipe or closing brace
		for p.pos < len(p.input) {
			if p.current() == '|' || (p.pos+1 < len(p.input) && p.input[p.pos:p.pos+2] == closeBrace) {
				break
			}
			p.pos++
		}
	}

	// Ensure proper closing
	p.skipWhitespace()
	if !p.match(closeBrace) {
		return nil
	}

	return &ValueRef{
		Path:         path,
		DefaultValue: defaultValue,
		SourceFile:   p.template,
		LineNumber:   p.lineNum,
	}
}

// parseValuePath parses the dot-notation path after .Values.
func (p *parser) parseValuePath() string {
	var path strings.Builder
	lastWasDot := true // Start with true to prevent leading dot

	for p.pos < len(p.input) {
		ch := p.current()
		if ch == '.' {
			if lastWasDot {
				return "" // Invalid: consecutive dots
			}
			lastWasDot = true
		} else if isValidPathChar(ch) {
			lastWasDot = false
		} else {
			break
		}

		path.WriteByte(ch)
		p.pos++
	}

	// Check if path ends with a dot
	if lastWasDot {
		return ""
	}

	return path.String()
}

// parseDefaultValue parses the default value after the default function
func (p *parser) parseDefaultValue() string {
	p.skipWhitespace()

	// Handle quoted strings
	switch p.current() {
	case '"', '\'':
		quote := p.current()
		p.pos++
		var value strings.Builder
		escaped := false

		for p.pos < len(p.input) {
			ch := p.current()
			if escaped {
				value.WriteByte(ch)
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == quote && !escaped {
				p.pos++ // Skip closing quote
				return value.String()
			} else {
				value.WriteByte(ch)
			}
			p.pos++
		}
		return "" // Unclosed quote

	// Handle numeric values
	default:
		var value strings.Builder
		for p.pos < len(p.input) && (isDigit(p.current()) || p.current() == '.') {
			value.WriteByte(p.current())
			p.pos++
		}
		return value.String()
	}
}

// Helper methods
func (p *parser) current() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *parser) match(s string) bool {
	if p.pos+len(s) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(s)] == s {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && isWhitespace(p.current()) {
		if p.current() == '\n' {
			p.lineNum++
		}
		p.pos++
	}
}

func isValidPathChar(ch byte) bool {
	return isAlphaNumeric(ch) || ch == '.' || ch == '-' || ch == '_'
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isAlphaNumeric(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
