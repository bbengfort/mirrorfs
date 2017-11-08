package mirrorfs

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
)

//===========================================================================
// Initialization
//===========================================================================

// Initialize the package and random numbers, etc.
func init() {
	// Initialize our debug logging with our prefix
	logger = log.New(os.Stdout, "[mirrorfs] ", log.Lmicroseconds)
	cautionCounter = new(counter)
	cautionCounter.init()
}

func signalHandler(mount string) {
	// Make signal channel and register notifiers for Interupt and Terminate
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	signal.Notify(sigchan, syscall.SIGTERM)

	// Block until we receive a signal on the channel
	<-sigchan

	// Defer the clean exit until the end of the function
	defer os.Exit(0)

	// Ensure the file system is unmounted
	err := fuse.Unmount(mount)
	if err != nil {
		warn("unmount error: %s", err)
		os.Exit(1)
	}
	info("unmounted %s", mount)
}

//===========================================================================
// Global Entry Point to FUSE mount
//===========================================================================

// Mount the mirror file system at the specified path, mirroring to the other
// path. This returns an error if the mount point does not exist.
func Mount(mount, mirror string) (err error) {
	info("mounting %s to mirror %s", mount, mirror)

	// Unmount the FS in case it was mounted with errors
	fuse.Unmount(mount)

	// Mount the FS with the specified options.
	fs := NewFS(mount, mirror, false)
	conn, err := fuse.Mount(
		mount,
		fuse.FSName("MirrorFS"),
		fuse.Subtype("mirrorfs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Mirror Volume"),
	)
	if err != nil {
		return err
	}

	// Ensure connection is closed when done
	defer conn.Close()

	// Ensure that we unmount the file system when done
	go signalHandler(mount)

	// Serve the file system.
	if err := fusefs.Serve(conn, fs); err != nil {
		return err
	}

	// Block until mount process has an error to report.
	<-conn.Ready
	if conn.MountError != nil {
		return conn.MountError
	}

	return nil
}

//===========================================================================
// File System
//===========================================================================

// NewFS returns a new FileSystem object
func NewFS(mount, mirror string, abs bool) *FileSystem {
	if abs {
		mount, _ = filepath.Abs(mount)
		mirror, _ = filepath.Abs(mirror)
	}

	fs := new(FileSystem)
	fs.mount = mount
	fs.mirror = mirror
	fs.root, _ = fs.makeNode(fs.mount)
	return fs
}

// FileSystem implements fusefs.FS* interfaces.
type FileSystem struct {
	mount  string // Location of the mount point
	mirror string // Location to mirror operations to
	root   *Node  // Node of the root directory
}

// Root implements fusefs.FS
func (fs FileSystem) Root() (fusefs.Node, error) {
	return fs.root, nil
}

// create a node from a path relative to the mount directory.
func (fs *FileSystem) makeNode(path string) (*Node, error) {
	return &Node{path, fs, nil}, nil
}
