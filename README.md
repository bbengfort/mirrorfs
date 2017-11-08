# MirrorFS

**A simple FUSE file system that mirrors operations from the mounted directory to another directory.**

## Usage

Clone the repository into your `$GOPATH` or install as follows:

```
$ go get github.com/bbengfort/... 
```

If you've run `go install` (by default with `go get`) then you should have the `mirrorfs` command:

```
$ mirrorfs mount path/to/mount path/to/mirror 
```

All operations in `path/to/mount` will be mirrored to `path/to/mirror` and vice-versa. 


## Fuse Implementation

The `bazil.org/fuse` implementation has many interfaces for various FUSE requests and responses that may overlap. For example `ReadAll` will supersede `Read` if implemented ([`serve.go L1228`](https://github.com/bazil/fuse/blob/371fbbdaa8987b715bdd21d6adc4c9b20155f748/fs/serve.go#L1228)). A full inspection of the [`handleRequest`](https://github.com/bazil/fuse/blob/371fbbdaa8987b715bdd21d6adc4c9b20155f748/fs/serve.go#L907) method is required to understand the full FUSE response handling by the Bazil implementation.

MirrorFS implements the following methods/interfaces:

- `(*FileSystem) Root() (fusefs.Node, error)`
- `(*Node) Attr(ctx context.Context, attr *fuse.Attr) error`
- `(*Node) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error`
- `(*Node) Lookup(ctx context.Context, name string) (fusefs.Node, error)`
- `(*Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error)`
- `(*Node) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fusefs.Node, error)`
- `(*Node) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error)`
- `(*Node) Remove(ctx context.Context, req *fuse.RemoveRequest) error`
- `(*Node) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fusefs.Node) error`
- `(*Node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error`
- `(*Node) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error`
- `(*Node) Fsync(ctx context.Context, req *fuse.FsyncRequest) error`
- `(*Node) Flush(ctx context.Context, req *fuse.FlushRequest) error`
- `(*Node) Release(ctx context.Context, req *fuse.ReleaseRequest) error`

### Notes

- If `Getattr` is not implemented will use `Attr` and fill in zero values.
- `Setattr` is used to communicate changes to a file size, e.g. truncate.
- `Create` creates and opens a file handle; the node is also used as a handle since it has an open reference to the file object in mirror.

### Reference

This section contains a complete listing of interfaces that can be implemented by a bazil.org/fuse application.

#### FS Interfaces

- `Root() (Node, error)`
- `Destroy()`
- `GenerateInode(parentInode uint64, name string) uint64`
- `Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error`

#### Node Interfaces

- `Attr(ctx context.Context, attr *fuse.Attr) error`
- `Access(ctx context.Context, req *fuse.AccessRequest) error`
- `Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (Node, Handle, error)`
- `Forget()`
- `Fsync(ctx context.Context, req *fuse.FsyncRequest) error`
- `Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error`
- `Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error`
- `Link(ctx context.Context, req *fuse.LinkRequest, old Node) (Node, error)`
- `Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error`
- `Mkdir(ctx context.Context, req *fuse.MkdirRequest) (Node, error)`
- `Mknod(ctx context.Context, req *fuse.MknodRequest) (Node, error)`
- `Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (Handle, error)`
- `Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error)`
- `Remove(ctx context.Context, req *fuse.RemoveRequest) error`
- `Removexattr(ctx context.Context, req *fuse.RemovexattrRequest) error`
- `Rename(ctx context.Context, req *fuse.RenameRequest, newDir Node) error`
- `Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (Node, error)`
- `Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error`
- `Setxattr(ctx context.Context, req *fuse.SetxattrRequest) error`
- `Lookup(ctx context.Context, name string) (Node, error)`
- `Symlink(ctx context.Context, req *fuse.SymlinkRequest) (Node, error)`

#### Handle Interfaces

- `Flush(ctx context.Context, req *fuse.FlushRequest) error`
- `ReadAll(ctx context.Context) ([]byte, error)`
- `ReadDirAll(ctx context.Context) ([]fuse.Dirent, error)`
- `Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error`
- `Release(ctx context.Context, req *fuse.ReleaseRequest) error`
- `Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error`
