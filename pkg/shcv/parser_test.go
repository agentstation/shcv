package shcv

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLineBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		template string
		want     []ValueRef
	}{
		{
			name:     "simple value reference",
			input:    "{{ .Values.simple }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "simple", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "value with default",
			input:    "{{ .Values.key | default \"defaultValue\" }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "key", DefaultValue: "defaultValue", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "multiple values in one line",
			input:    "{{ .Values.first }} and {{ .Values.second }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "first", SourceFile: "test.yaml", LineNumber: 1},
				{Path: "second", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "nested path",
			input:    "{{ .Values.parent.child }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "parent.child", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "numeric default value",
			input:    "{{ .Values.port | default 8080 }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "port", DefaultValue: "8080", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "multiple lines",
			input:    "{{ .Values.first }}\n{{ .Values.second }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "first", SourceFile: "test.yaml", LineNumber: 1},
				{Path: "second", SourceFile: "test.yaml", LineNumber: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newParser(tt.input, tt.template)
			got := p.parse()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLineEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		template string
		want     []ValueRef
	}{
		{
			name:     "empty input",
			input:    "",
			template: "test.yaml",
			want:     nil,
		},
		{
			name:     "no values reference",
			input:    "{{ .Chart.Name }}",
			template: "test.yaml",
			want:     nil,
		},
		{
			name:     "incomplete braces",
			input:    "{{ .Values.incomplete",
			template: "test.yaml",
			want:     nil,
		},
		{
			name:     "whitespace variations",
			input:    "{{    .Values.spaced   |   default   \"value\"    }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "spaced", DefaultValue: "value", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "special characters in path",
			input:    "{{ .Values.my-key_name.sub-key }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "my-key_name.sub-key", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newParser(tt.input, tt.template)
			got := p.parse()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseValueRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		template string
		want     *ValueRef
	}{
		{
			name:     "simple value",
			input:    "{{ .Values.key }}",
			template: "test.yaml",
			want:     &ValueRef{Path: "key", SourceFile: "test.yaml", LineNumber: 1},
		},
		{
			name:     "with default string",
			input:    "{{ .Values.key | default \"value\" }}",
			template: "test.yaml",
			want:     &ValueRef{Path: "key", DefaultValue: "value", SourceFile: "test.yaml", LineNumber: 1},
		},
		{
			name:     "with single quotes",
			input:    "{{ .Values.key | default 'value' }}",
			template: "test.yaml",
			want:     &ValueRef{Path: "key", DefaultValue: "value", SourceFile: "test.yaml", LineNumber: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newParser(tt.input, tt.template)
			p.match("{{") // Move past opening braces
			got := p.parseValueRef()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseValueRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLineAdvancedCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		template string
		want     []ValueRef
	}{
		{
			name:     "escaped quotes in default",
			input:    `{{ .Values.key | default "value \"quoted\" here" }}`,
			template: "test.yaml",
			want: []ValueRef{
				{Path: "key", DefaultValue: `value "quoted" here`, SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "very long path",
			input:    "{{ .Values.this.is.a.very.long.nested.path.that.should.still.work }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "this.is.a.very.long.nested.path.that.should.still.work", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "multiple pipes",
			input:    "{{ .Values.key | default \"\" | quote }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "key", DefaultValue: "", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
		{
			name:     "mixed quotes",
			input:    "{{ .Values.key | default \"value's here\" }}",
			template: "test.yaml",
			want: []ValueRef{
				{Path: "key", DefaultValue: "value's here", SourceFile: "test.yaml", LineNumber: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newParser(tt.input, tt.template)
			got := p.parse()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLineMalformedCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		template string
		want     []ValueRef
	}{
		{
			name:     "unclosed quotes",
			input:    `{{ .Values.key | default "unclosed }}`,
			template: "test.yaml",
			want:     nil,
		},
		{
			name:     "missing closing brace",
			input:    "{{ .Values.key | default \"value\" }",
			template: "test.yaml",
			want:     nil,
		},
		{
			name:     "invalid path characters",
			input:    "{{ .Values.key@invalid }}",
			template: "test.yaml",
			want:     nil,
		},
		{
			name:     "empty path",
			input:    "{{ .Values. }}",
			template: "test.yaml",
			want:     nil,
		},
		{
			name:     "multiple consecutive dots",
			input:    "{{ .Values..key }}",
			template: "test.yaml",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newParser(tt.input, tt.template)
			got := p.parse()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		templatePath string
		want         []ValueRef
	}{
		{
			name:         "simple value reference",
			content:      "{{ .Values.simple }}",
			templatePath: "test.yaml",
			want: []ValueRef{
				{
					Path:       "simple",
					SourceFile: "test.yaml",
					LineNumber: 1,
				},
			},
		},
		{
			name:         "value with default",
			content:      "{{ .Values.withDefault | default \"defaultValue\" }}",
			templatePath: "test.yaml",
			want: []ValueRef{
				{
					Path:         "withDefault",
					SourceFile:   "test.yaml",
					LineNumber:   1,
					DefaultValue: "defaultValue",
				},
			},
		},
		{
			name:         "multiple values",
			content:      "{{ .Values.first }}\n{{ .Values.second }}",
			templatePath: "test.yaml",
			want: []ValueRef{
				{
					Path:       "first",
					SourceFile: "test.yaml",
					LineNumber: 1,
				},
				{
					Path:       "second",
					SourceFile: "test.yaml",
					LineNumber: 2,
				},
			},
		},
		{
			name:         "nested value",
			content:      "{{ .Values.parent.child }}",
			templatePath: "test.yaml",
			want: []ValueRef{
				{
					Path:       "parent.child",
					SourceFile: "test.yaml",
					LineNumber: 1,
				},
			},
		},
		{
			name:         "no values",
			content:      "just some text\nwithout any values",
			templatePath: "test.yaml",
			want:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFile(tt.content, tt.templatePath)
			if len(got) == 0 && tt.want == nil {
				return // both empty, test passes
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParserHelpers(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
			pos   int
			want  byte
		}{
			{
				name:  "first character",
				input: "test",
				pos:   0,
				want:  't',
			},
			{
				name:  "middle character",
				input: "test",
				pos:   2,
				want:  's',
			},
			{
				name:  "last character",
				input: "test",
				pos:   3,
				want:  't',
			},
			{
				name:  "beyond end",
				input: "test",
				pos:   4,
				want:  0,
			},
			{
				name:  "empty input",
				input: "",
				pos:   0,
				want:  0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := &parser{input: tt.input, pos: tt.pos}
				got := p.current()
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("skipWhitespace", func(t *testing.T) {
		tests := []struct {
			name      string
			input     string
			startPos  int
			wantPos   int
			wantLines int
		}{
			{
				name:      "no whitespace",
				input:     "test",
				startPos:  0,
				wantPos:   0,
				wantLines: 1,
			},
			{
				name:      "spaces",
				input:     "   test",
				startPos:  0,
				wantPos:   3,
				wantLines: 1,
			},
			{
				name:      "tabs",
				input:     "\t\ttest",
				startPos:  0,
				wantPos:   2,
				wantLines: 1,
			},
			{
				name:      "newlines",
				input:     "\n\ntest",
				startPos:  0,
				wantPos:   2,
				wantLines: 3,
			},
			{
				name:      "mixed whitespace",
				input:     " \t\n\r test",
				startPos:  0,
				wantPos:   5,
				wantLines: 2,
			},
			{
				name:      "at end",
				input:     "test   ",
				startPos:  4,
				wantPos:   7,
				wantLines: 1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				p := &parser{input: tt.input, pos: tt.startPos, lineNum: 1}
				p.skipWhitespace()
				assert.Equal(t, tt.wantPos, p.pos)
				assert.Equal(t, tt.wantLines, p.lineNum)
			})
		}
	})

	t.Run("character checks", func(t *testing.T) {
		// Test isValidPathChar
		validPathChars := []byte("abcZY09.-_")
		invalidPathChars := []byte("!@#$%^&*()")
		for _, ch := range validPathChars {
			assert.True(t, isValidPathChar(ch), "char %c should be valid", ch)
		}
		for _, ch := range invalidPathChars {
			assert.False(t, isValidPathChar(ch), "char %c should be invalid", ch)
		}

		// Test isWhitespace
		whitespaceChars := []byte{' ', '\t', '\n', '\r'}
		nonWhitespaceChars := []byte{'a', '1', '.', '-'}
		for _, ch := range whitespaceChars {
			assert.True(t, isWhitespace(ch), "char %c should be whitespace", ch)
		}
		for _, ch := range nonWhitespaceChars {
			assert.False(t, isWhitespace(ch), "char %c should not be whitespace", ch)
		}

		// Test isAlphaNumeric
		alphaNumChars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
		nonAlphaNumChars := []byte("!@#$%^&*()_+-=[]{}|;:,.<>?")
		for _, ch := range alphaNumChars {
			assert.True(t, isAlphaNumeric(ch), "char %c should be alphanumeric", ch)
		}
		for _, ch := range nonAlphaNumChars {
			assert.False(t, isAlphaNumeric(ch), "char %c should not be alphanumeric", ch)
		}

		// Test isDigit
		digitChars := []byte("0123456789")
		nonDigitChars := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%^&*()")
		for _, ch := range digitChars {
			assert.True(t, isDigit(ch), "char %c should be digit", ch)
		}
		for _, ch := range nonDigitChars {
			assert.False(t, isDigit(ch), "char %c should not be digit", ch)
		}
	})
}
