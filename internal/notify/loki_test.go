package notify

import (
	"reflect"
	"testing"
)

func TestParseLokiLabels(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"single", "job=pgwd", map[string]string{"job": "pgwd"}},
		{"two", "job=pgwd,env=prod", map[string]string{"job": "pgwd", "env": "prod"}},
		{"spaces", " job = pgwd , env = prod ", map[string]string{"job": "pgwd", "env": "prod"}},
		{"empty value", "key=", map[string]string{"key": ""}},
		{"value with equals", "k=a=b", map[string]string{"k": "a=b"}},
		{"no equals", "justkey", map[string]string{}},
		{"comma only", ",,,", map[string]string{}},
		{"many", "a=1,b=2,c=3", map[string]string{"a": "1", "b": "2", "c": "3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLokiLabels(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLokiLabels(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
