package mirrorfs

import (
	"os"
	"syscall"
	"time"

	"bazil.org/fuse"
)

// Converts a syscall time into a golang time
func timefspec(ts syscall.Timespec) time.Time {
	sec, nsec := ts.Unix()
	return time.Unix(sec, nsec)
}

// Converts an os.PathError to a FUSE error
func errno(err error) fuse.Errno {
	if os.IsExist(err) {
		debug(err.Error())
		return fuse.EEXIST
	}

	if os.IsNotExist(err) {
		debug(err.Error())
		return fuse.ENOENT
	}

	if os.IsPermission(err) {
		debug(err.Error())
		return fuse.EPERM
	}

	warne(err) // Unknown error has occurred
	return fuse.DefaultErrno
}

// Checks to see if a path exists
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// Returns the fuse type from a stat response
func fuseType(info os.FileInfo) fuse.DirentType {
	if info.IsDir() {
		return fuse.DT_Dir
	}
	return fuse.DT_File
}
