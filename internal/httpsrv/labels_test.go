package httpsrv

import "testing"

func TestEscapePrometheusLabelValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "prod", "prod"},
		{"backslash", `a\b`, `a\\b`},
		{"quote", `say"hi`, `say\"hi`},
		{"newline", "a\nb", `a\nb`},
		{"cr stripped", "a\rb", "ab"},
		{"control char", "a\x00b", "a_b"},
		{"combined", "cl\"us\ner", `cl\"us\ner`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapePrometheusLabelValue(tt.input); got != tt.want {
				t.Errorf("escapePrometheusLabelValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPromLabel(t *testing.T) {
	got := promLabel("client", `x"y`)
	want := `client="x\"y"`
	if got != want {
		t.Errorf("promLabel = %q, want %q", got, want)
	}
}
