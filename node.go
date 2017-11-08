package mirrorfs

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Node implements the fuse.Node methods.
type Node struct {
	path string      // path to location relative to mountpoint and mirror.
	fs   *FileSystem // reference to file system the node belongs to.
}

//===========================================================================
// Helper Functions
//===========================================================================

// Returns the fuse type from a stat response
func fuseType(info os.FileInfo) fuse.DirentType {
	if info.IsDir() {
		return fuse.DT_Dir
	}
	return fuse.DT_File
}

// Find the mirror path according to the mirror directory in the file system.
func (n *Node) mirrorPath() string {
	rel, _ := filepath.Rel(n.fs.mount, n.path)
	return filepath.Join(n.fs.mirror, rel)
}

// Returns both the file info and the system stats for a node
func (n *Node) info() (os.FileInfo, *syscall.Stat_t, error) {
	finfo, err := os.Stat(n.mirrorPath())
	if err != nil {
		return nil, nil, err
	}

	stat := finfo.Sys().(*syscall.Stat_t)
	return finfo, stat, nil
}

//===========================================================================
// Common Node Methods
//===========================================================================

// Attr implements the fuse.Node interface (also used for Getattr)
func (n *Node) Attr(ctx context.Context, attr *fuse.Attr) error {

	now := time.Now()
	finfo, stat, err := n.info()
	if err != nil {
		// What error should we return here?
		return err
	}

	attr.Inode = stat.Ino            // inode number -- currently unknown
	attr.Size = uint64(finfo.Size()) // size in bytes
	attr.Uid = stat.Uid              // owner uid
	attr.Gid = stat.Gid              // group gid
	attr.Mode = finfo.Mode()         // file mode
	attr.Atime = now                 // time of last access
	attr.Mtime = finfo.ModTime()     // time of last modification
	// attr.Ctime = now             // time of last inode change
	// attr.Crtime = now            // time of creation (OS X only)
	// attr.Nlink = 1               // number of links (usually 1)

	// attr.Rdev = 0                // device numbers
	// attr.Flags = 0               // chflags(2) flags (OS X only)
	// attr.Blocks = 0              // size in 512-byte units
	// attr.BlockSize = 512         // size of blocks on disk

	return nil
}

// Setattr implements the fuse.NodeSetattrer interface
func (n *Node) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {

	finfo, stat, err := n.info()
	if err != nil {
		// What error should we return here?
		return err
	}

	if req.Valid.Size() {
		// Truncate the node if it's a file.
		if !finfo.IsDir() {
			if err := os.Truncate(n.mirrorPath(), int64(req.Size)); err != nil {
				return err
			}
		} else {
			caution("attempting to truncate a directory?!")
		}
	}

	if req.Valid.Uid() || req.Valid.Gid() {
		// Execute chown on the object
		var uid uint32
		var gid uint32
		if req.Valid.Uid() {
			uid = req.Uid
		} else {
			uid = stat.Uid
		}

		if req.Valid.Gid() {
			gid = req.Gid
		} else {
			gid = stat.Gid
		}

		if err := os.Chown(n.mirrorPath(), int(uid), int(gid)); err != nil {
			return err
		}
	}

	if req.Valid.Mode() {
		// Execute chmod on the object
		os.Chmod(n.mirrorPath(), req.Mode)
	}

	if req.Valid.Atime() || req.Valid.AtimeNow() || req.Valid.Mtime() || req.Valid.MtimeNow() {
		var atime time.Time
		var mtime time.Time
		if req.Valid.AtimeNow() {
			atime = time.Now()
		} else if req.Valid.Atime() {
			atime = req.Atime
		} else {
			atime = timefspec(stat.Atimespec)
		}

		if req.Valid.MtimeNow() {
			mtime = time.Now()
		} else if req.Valid.Mtime() {
			mtime = req.Mtime
		} else {
			mtime = timefspec((stat.Mtimespec))
		}

		os.Chtimes(n.mirrorPath(), atime, mtime)
	}

	// Unhandled attributes
	if req.Valid.Handle() {
		debug("ignoring setting handle on node %d", n.path)
	}

	if req.Valid.LockOwner() {
		debug("ignoring setting lock owner on node %d", n.path)
	}

	if req.Valid.Bkuptime() {
		debug("ignoring setting bkuptime on node %d", n.path)
	}

	if req.Valid.Chgtime() {
		debug("ignoring setting chgtime on node %d", n.path)
	}

	if req.Valid.Crtime() {
		debug("ignoring setting crtime on node %d", n.path)
	}

	if req.Valid.Flags() {
		debug("ignoring setting flags on node %d", n.path)
	}

	// VERY IMPORANT! Set the new attrs on the response!
	return n.Attr(ctx, &resp.Attr)
}

//===========================================================================
// Directory Node Methods
//===========================================================================

// Lookup implements the fuse.NodeRequestLookuper interface.
func (n *Node) Lookup(ctx context.Context, name string) (fs.Node, error) {
	// Create a node for the given name in the directory
	path := filepath.Join(n.path, name)
	return n.fs.makeNode(path)
}

// ReadDirAll implements the fuse.HandleReadDirAller interface.
func (n *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {

	// List the contents of the mirror path
	finfos, err := ioutil.ReadDir(n.mirrorPath())
	if err != nil {
		// What error should we return here?
		return nil, err
	}

	// Create the listing response
	ents := make([]fuse.Dirent, len(finfos))

	// Return fuse directory entities for listing
	for idx, finfo := range finfos {
		stat := finfo.Sys().(*syscall.Stat_t)
		ents[idx] = fuse.Dirent{
			Inode: stat.Ino,
			Type:  fuseType(finfo),
			Name:  finfo.Name(),
		}
	}

	return ents, nil
}

//===========================================================================
// File Node Methods
//===========================================================================
