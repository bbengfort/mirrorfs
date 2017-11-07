package mirrorfs

import (
	"context"
	"os"
	"path/filepath"

	"bazil.org/fuse"
)

// Node implements the fuse.Node methods.
type Node struct {
	path string      // path to location relative to mountpoint and mirror.
	fs   *FileSystem // reference to file system the node belongs to.
}

// Find the mirror path according to the mirror directory in the file system.
func (n *Node) mirrorPath() string {
	rel, _ := filepath.Rel(n.fs.mount, n.path)
	return filepath.Join(n.fs.mirror, rel)
}

// Attr implements the fuse.Node interface
func (n *Node) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = 1
	attr.Mode = os.ModeDir | 0555
	return nil
}

// ReadDirAll implements the fuse.NodeReadDirAller interface
func (n *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	info(n.mirrorPath())
	return []fuse.Dirent{}, nil
}
