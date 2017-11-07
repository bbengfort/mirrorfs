package mirrorfs

import (
	"log"
	"os"

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
}

//===========================================================================
// Global Entry Point to FUSE mount
//===========================================================================

// Mount the mirror file system at the specified path, mirroring to the other
// path. This returns an error if the mount point does not exist.
func Mount(mount, mirror string) (err error) {
	info("mounting mirrorfs://%s to mirror to %s", mount, mirror)

	// Unmount the FS in case it was mounted with errors
	fuse.Unmount(mount)

	// Mount the FS with the specified options.
	fs := NewFS(mount, mirror)
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
func NewFS(mount, mirror string) *FileSystem {
	fs := new(FileSystem)
	fs.mount = mount
	fs.mirror = mirror
	fs.root, _ = fs.makeNode(mount)
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
	return &Node{path, fs}, nil
}