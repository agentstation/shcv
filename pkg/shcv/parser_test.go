package shcv

import (
	"reflect"
	"testing"
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
