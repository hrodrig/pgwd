package contrib

import (
	"strings"
	"testing"
)

func TestSampleConf(t *testing.T) {
	s := SampleConf()
	if !strings.Contains(s, "client:") {
		t.Fatal("missing client key")
	}
	if !strings.Contains(s, "databases:") {
		t.Fatal("missing databases block")
	}
}
