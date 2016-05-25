package transport

import "strings"

// unfortunately net.errClosing is not exported
func isNetErrClosing(err error) bool {
	if err == nil {
		return false
	}

	return strings.HasSuffix(err.Error(), "use of closed network connection")
}
