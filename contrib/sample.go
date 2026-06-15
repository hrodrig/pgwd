package contrib

import _ "embed"

//go:embed pgwd.conf.example
var sampleConf []byte

// SampleConf returns the annotated example configuration (same as contrib/pgwd.conf.example).
func SampleConf() string {
	return string(sampleConf)
}
