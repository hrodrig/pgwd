package notify

import (
	"errors"
	"net/url"
)

// RedactNetError returns a copy of err with *url.Error.URL reduced to scheme://host
// so webhook paths and query secrets are not logged. Non-URL errors pass through.
func RedactNetError(err error) error {
	if err == nil {
		return nil
	}
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	safeURL := "***"
	if u, parseErr := url.Parse(ue.URL); parseErr == nil && u.Host != "" {
		safeURL = u.Scheme + "://" + u.Host
	}
	return &url.Error{Op: ue.Op, URL: safeURL, Err: ue.Err}
}
