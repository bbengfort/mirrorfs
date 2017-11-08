package mirrorfs

import (
	"syscall"
	"time"
)

// Converts a syscall time into a golang time
func timefspec(ts syscall.Timespec) time.Time {
	sec, nsec := ts.Unix()
	return time.Unix(sec, nsec)
}
