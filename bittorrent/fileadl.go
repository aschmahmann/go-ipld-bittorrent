package bittorrent

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/adl"
	"github.com/ipld/go-ipld-prime/datamodel"

	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
)

type LargeBytesNodeADL interface {
	datamodel.LargeBytesNode
	adl.ADL
}

func NewBTFile(ctx context.Context, substrate ipld.Node, lsys *ipld.LinkSystem) (LargeBytesNodeADL, error) {
	return &btfileNode{
		ctx:       ctx,
		lsys:      lsys,
		substrate: substrate,
	}, nil
}

type btfileNode struct {
	ctx       context.Context
	lsys      *ipld.LinkSystem
	substrate ipld.Node
}

var _ LargeBytesNodeADL = (*btfileNode)(nil)

func (b *btfileNode) Substrate() datamodel.Node {
	return b.substrate
}

func (b *btfileNode) Kind() datamodel.Kind {
	return ipld.Kind_Bytes
}

func (b *btfileNode) LookupByString(key string) (datamodel.Node, error) {
	return nil, ipld.ErrWrongKind{}
}

func (b *btfileNode) LookupByNode(key datamodel.Node) (datamodel.Node, error) {
	return nil, ipld.ErrWrongKind{}
}

func (b *btfileNode) LookupByIndex(idx int64) (datamodel.Node, error) {
	return nil, ipld.ErrWrongKind{}
}

func (b *btfileNode) LookupBySegment(seg datamodel.PathSegment) (datamodel.Node, error) {
	return nil, ipld.ErrWrongKind{}
}

func (b *btfileNode) MapIterator() datamodel.MapIterator {
	return nil
}

func (b *btfileNode) ListIterator() datamodel.ListIterator {
	return nil
}

func (b *btfileNode) Length() int64 {
	return -1
}

func (b *btfileNode) IsAbsent() bool {
	return false
}

func (b *btfileNode) IsNull() bool {
	panic("implement me")
}

func (b *btfileNode) AsBool() (bool, error) {
	return false, ipld.ErrWrongKind{TypeName: "bool", MethodName: "AsBool", AppropriateKind: ipld.KindSet_JustBytes}
}

func (b *btfileNode) AsInt() (int64, error) {
	return 0, ipld.ErrWrongKind{TypeName: "int", MethodName: "AsInt", AppropriateKind: ipld.KindSet_JustBytes}
}

func (b *btfileNode) AsFloat() (float64, error) {
	return 0, ipld.ErrWrongKind{TypeName: "float", MethodName: "AsFloat", AppropriateKind: ipld.KindSet_JustBytes}
}

func (b *btfileNode) AsString() (string, error) {
	return "", ipld.ErrWrongKind{TypeName: "string", MethodName: "AsString", AppropriateKind: ipld.KindSet_JustBytes}
}

func (b *btfileNode) AsBytes() ([]byte, error) {
	rdr, err := b.AsLargeBytes()
	if err != nil {
		return nil, err
	}
	return io.ReadAll(rdr)
}

func (b *btfileNode) AsLink() (datamodel.Link, error) {
	return nil, ipld.ErrWrongKind{TypeName: "link", MethodName: "AsLink", AppropriateKind: ipld.KindSet_JustBytes}
}

func (b *btfileNode) Prototype() datamodel.NodePrototype {
	return nil
}

func (b *btfileNode) AsLargeBytes() (io.ReadSeeker, error) {
	return &fileReader{b, nil, 0}, nil
}

type fileReader struct {
	*btfileNode
	rdr    io.Reader
	offset int64
}

var _ io.ReadSeeker = (*fileReader)(nil)

func (f *fileReader) makeReader() (io.Reader, error) {
	linksNode, err := f.substrate.LookupByString("pieces")
	if err != nil {
		return nil, err
	}

	// pieces are supposed to a concatenated set of 20-byte SHA1 bytestrings
	linksStr, err := linksNode.AsString()
	if err != nil {
		return nil, err
	}

	const hashlen = 20
	if len(linksStr) % hashlen != 0 {
		return nil, fmt.Errorf("pieces string is not a multiple of 20 bytes")
	}

	pieceLenNode, err := f.substrate.LookupByString("piece length")
	if err != nil {
		return nil, err
	}
	pieceLen, err := pieceLenNode.AsInt()
	if err != nil {
		return nil, err
	}

	fileLenNode, err := f.substrate.LookupByString("length")
	if err != nil {
		return nil, err
	}
	fileLen, err := fileLenNode.AsInt()
	if err != nil {
		return nil, err
	}

	// skip if past the end
	if f.offset >= fileLen {
		return nil, fmt.Errorf("tried reading past the end of the file")
	}

	readers := make([]io.Reader, 0)
	at := int64(0)
	for i := 0; i < len(linksStr); i+=int(hashlen) {
		if f.offset > at+pieceLen {
			at += pieceLen
			continue
		}

		singleLinkStr := linksStr[i:i+20]
		mh, err := multihash.Encode([]byte(singleLinkStr), multihash.SHA1)
		if err != nil {
			return nil, err
		}
		lnklnk := cidlink.Link{Cid: cid.NewCidV1(cid.Raw, mh)}

		tr := newDeferredRawNodeReader(f.ctx, f.lsys, lnklnk)

		// fastforward the first one if needed.
		if at < f.offset {
			_, err := tr.Seek(f.offset-at, io.SeekStart)
			if err != nil {
				return nil, err
			}
		}
		at += pieceLen
		readers = append(readers, tr)
	}
	if len(readers) == 0 {
		return nil, io.EOF
	}
	return io.MultiReader(readers...), nil
}

func (f *fileReader) Read(p []byte) (int, error) {
	// build reader
	if f.rdr == nil {
		rdr, err := f.makeReader()
		if err != nil {
			return 0, err
		}
		f.rdr = rdr
	}
	n, err := f.rdr.Read(p)
	return n, err
}

func (f *fileReader) Seek(offset int64, whence int) (int64, error) {
	if f.rdr != nil {
		f.rdr = nil
	}
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		lenNode, err := f.substrate.LookupByString("length")
		if err != nil {
			return 0, err
		}
		lenFile, err := lenNode.AsInt()
		if err != nil {
			return 0, err
		}
		f.offset = lenFile + offset
	}
	return f.offset, nil
}

func ReifyBTFile(lnkCtx ipld.LinkContext, maybeBTFile ipld.Node, lsys *ipld.LinkSystem) (ipld.Node, error) {
	btf, err := NewBTFile(lnkCtx.Ctx, maybeBTFile, lsys)
	if err != nil {
		return nil, err
	}
	return btf, nil
}

func newDeferredRawNodeReader(ctx context.Context, lsys *ipld.LinkSystem, root ipld.Link) io.ReadSeeker {
	dfn := deferredRawNode{
		root:           root,
		lsys:           lsys,
		ctx:            ctx,
	}
	return &dfn
}

type deferredRawNode struct {
	root ipld.Link
	lsys *ipld.LinkSystem
	ctx  context.Context

	rs io.ReadSeeker
}

func (d *deferredRawNode) resolve() error {
	if d.lsys == nil {
		return nil
	}

	target, err := d.lsys.Load(ipld.LinkContext{Ctx: d.ctx}, d.root, basicnode.Prototype__Bytes{})
	if err != nil {
		return err
	}

	lbn, ok := target.(datamodel.LargeBytesNode)
	if !ok {
		return fmt.Errorf("target node does not support the LargeBytesNode interface: %v", target)
	}
	rs, err := lbn.AsLargeBytes()
	if err != nil {
		return err
	}

	d.root = nil
	d.lsys = nil
	d.ctx = nil
	d.rs = rs

	return nil
}

func (d *deferredRawNode) Read(p []byte) (int, error) {
	if d.rs == nil {
		if err := d.resolve(); err != nil {
			return 0, err
		}
	}
	return d.rs.Read(p)
}

func (d *deferredRawNode) Seek(offset int64, whence int) (int64, error) {
	if d.rs == nil {
		if err := d.resolve(); err != nil {
			return 0, err
		}
	}
	return d.rs.Seek(offset, whence)
}
