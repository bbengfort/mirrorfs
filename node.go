package mirrorfs

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/net/context"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Node implements the fuse.Node methods.
type Node struct {
	path string      // path to location relative to mountpoint and mirror.
	fs   *FileSystem // reference to file system the node belongs to.
	file *os.File    // handle to an open file for reads and writes.
}

//===========================================================================
// Helper Functions
//===========================================================================

// Find the mirror path according to the mirror directory in the file system.
func (n *Node) mirrorPath() string {
	rel, _ := filepath.Rel(n.fs.mount, n.path)
	return filepath.Join(n.fs.mirror, rel)
}

// Returns both the file info and the system stat for a node
func (n *Node) info() (os.FileInfo, *syscall.Stat_t, error) {
	finfo, err := os.Stat(n.mirrorPath())
	if err != nil {
		return nil, nil, err
	}

	stat := finfo.Sys().(*syscall.Stat_t)
	return finfo, stat, nil
}

// Gets the parent (dirname) as a node from the current node
func (n *Node) parent() *Node {
	parent, _ := n.fs.makeNode(filepath.Dir(n.path))
	return parent
}

//===========================================================================
// Common Node Methods
//===========================================================================

// Attr implements the fuse.Node interface (also used for Getattr)
func (n *Node) Attr(ctx context.Context, attr *fuse.Attr) error {
	trace("Attr %s", n.path)

	now := time.Now()
	finfo, stat, err := n.info()
	if err != nil {
		return errno(err)
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
	trace("Setattr %s", n.path)

	finfo, stat, err := n.info()
	if err != nil {
		return errno(err)
	}

	if req.Valid.Size() {
		// Truncate the node if it's a file.
		if !finfo.IsDir() {
			debug("truncating %s to %d", n.path, req.Size)
			if err := os.Truncate(n.mirrorPath(), int64(req.Size)); err != nil {
				return errno(err)
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
			return errno(err)
		}
		debug("chown %s to %d:%d", n.path, uid, gid)
	}

	if req.Valid.Mode() {
		// Execute chmod on the object
		os.Chmod(n.mirrorPath(), req.Mode)
		debug("chmod %s to %s", n.path, req.Mode)
	}

	if req.Valid.Atime() || req.Valid.AtimeNow() || req.Valid.Mtime() || req.Valid.MtimeNow() {
		// Execute chtimes on the object
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
		debug("chtimes %s to atime %s and mtime %s", n.path, atime, mtime)
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
	trace("Lookup %s in %s", name, n.path)

	// Create a node for the given name in the directory
	path := filepath.Join(n.path, name)
	node, err := n.fs.makeNode(path)
	if err != nil {
		return nil, errno(err)
	}

	// Check to ensure the path exists in mirror
	if !pathExists(node.mirrorPath()) {
		return nil, fuse.ENOENT
	}

	return node, nil
}

// ReadDirAll implements the fuse.HandleReadDirAller interface.
func (n *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	trace("ReadDirAll %s", n.path)

	// List the contents of the mirror path
	finfos, err := ioutil.ReadDir(n.mirrorPath())
	if err != nil {
		return nil, errno(err)
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

// Mkdir implements fuse.NodeMkdirer
func (n *Node) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	trace("Mkdir %s in %s", req.Name, n.path)

	// Create the new filesystem node
	dir, err := n.fs.makeNode(filepath.Join(n.path, req.Name))
	if err != nil {
		return nil, errno(err)
	}

	// Make the directory in the mirror
	if err := os.Mkdir(dir.mirrorPath(), req.Mode); err != nil {
		return nil, errno(err)
	}

	// Chown the mirror directory according to the Uid and Gid of the caller
	os.Chown(dir.mirrorPath(), int(req.Header.Uid), int(req.Header.Gid))
	return dir, nil
}

// Create implements fuse.NodeCreater
func (n *Node) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	trace("Create %s in %s", req.Name, n.path)

	// Create the file node in the mount path
	var err error
	f, err := n.fs.makeNode(filepath.Join(n.path, req.Name))
	if err != nil {
		return nil, nil, errno(err)
	}

	// Open a handle to the file in the mirror path
	f.file, err = os.OpenFile(f.mirrorPath(), int(req.Flags), req.Mode)
	if err != nil {
		return nil, nil, errno(err)
	}

	// The node acts as an open file handle as well
	return f, f, nil
}

// Remove implements fuse.NodeRemover
func (n *Node) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	trace("Remove %s from %s", req.Name, n.path)

	path := filepath.Join(n.mirrorPath(), req.Name)
	if err := os.Remove(path); err != nil {
		return errno(err)
	}

	return nil
}

// Rename implements fuse.NodeRenamer
func (n *Node) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	d, ok := newDir.(*Node)
	if !ok {
		return errors.New("could not convert fs.Node to a mirrorfs.Node")
	}
	trace("Rename %s from %s to %s in %s", req.OldName, n.path, req.NewName, d.path)

	// Compute the source and destination paths for rename
	src := filepath.Join(n.mirrorPath(), req.OldName)
	dst := filepath.Join(d.mirrorPath(), req.NewName)

	if err := os.Rename(src, dst); err != nil {
		return errno(err)
	}

	return nil
}

//===========================================================================
// File Node Methods
//===========================================================================

// Read implements fuse.HandleReader
func (n *Node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) (err error) {
	trace("Read %s", n.path)

	if n.file == nil {
		// Find the mode of the file
		var info os.FileInfo
		info, err = os.Stat(n.mirrorPath())
		if err != nil {
			return errno(err)
		}

		// Open the file with the specified read flags
		n.file, err = os.OpenFile(n.mirrorPath(), int(req.FileFlags), info.Mode())
		if err != nil {
			return errno(err)
		}
	}

	resp.Data = make([]byte, req.Size)
	nbytes, err := n.file.ReadAt(resp.Data, req.Offset)
	if err != nil {
		if err != io.EOF {
			return errno(err)
		}

		// Otherwise modify the response to the exact length
		resp.Data = resp.Data[0:nbytes]
	}

	debug("read %d bytes from offest %d in %s", nbytes, req.Offset, n.path)
	return nil
}

// Write implements fuse.HandleWriter
func (n *Node) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) (err error) {
	trace("Write %s", n.path)

	if n.file == nil {
		// Find the mode of the file
		var info os.FileInfo
		info, err = os.Stat(n.mirrorPath())
		if err != nil {
			return errno(err)
		}

		// Open the file with the specified read flags
		n.file, err = os.OpenFile(n.mirrorPath(), int(req.FileFlags), info.Mode())
		if err != nil {
			return errno(err)
		}
	}

	// Write the data to the file
	resp.Size, err = n.file.WriteAt(req.Data, req.Offset)
	if err != nil {
		// TODO: when appending to a file currently getting a bad file
		// descriptor error. It appears that the append flag is not being set
		// which seems like a bug ...
		return errno(err)
	}

	debug("wrote %d bytes offset by %d to %s", resp.Size, req.Offset, n.path)
	return nil
}

// Fsync implements fuse.HandleFsyncer
func (n *Node) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	trace("Fsync %s", n.path)

	// fsync tells the OS to flush its buffers to the physical media
	if n.file != nil {
		if err := n.file.Sync(); err != nil {
			return errno(err)
		}
	}
	return nil
}

// Flush implments fuse.HandleFlusher
func (n *Node) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	trace("Flush %s", n.path)

	// flush the internal buffers of your application out to the OS
	debug("flush not implemented as there are no internal buffers")
	return nil
}

// Release implements fuse.HandleReleaser
func (n *Node) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	trace("Release %s", n.path)

	if n.file != nil {
		if req.ReleaseFlags == fuse.ReleaseFlush {
			if err := n.file.Sync(); err != nil {
				caution(err.Error())
			}
		}

		if err := n.file.Close(); err != nil {
			caution(err.Error())
		}

		// Ensure the handle is set to nil when closed successfully
		n.file = nil
	}
	return nil
}
