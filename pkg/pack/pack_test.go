package pack_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"testing"

	"github.com/restic/restic/pkg/backend/mem"
	"github.com/restic/restic/pkg/crypto"
	"github.com/restic/restic/pkg/pack"
	"github.com/restic/restic/pkg/restic"
	rtest "github.com/restic/restic/pkg/test"
)

var testLens = []int{23, 31650, 25860, 10928, 13769, 19862, 5211, 127, 13690, 30231}

type Buf struct {
	data []byte
	id   restic.ID
}

func newPack(t testing.TB, k *crypto.Key, lengths []int) ([]Buf, []byte, uint) {
	bufs := []Buf{}

	for _, l := range lengths {
		b := make([]byte, l)
		_, err := io.ReadFull(rand.Reader, b)
		rtest.OK(t, err)
		h := sha256.Sum256(b)
		bufs = append(bufs, Buf{data: b, id: h})
	}

	// pack blobs
	p := pack.NewPacker(k, new(bytes.Buffer))
	for _, b := range bufs {
		p.Add(restic.TreeBlob, b.id, b.data)
	}

	_, err := p.Finalize()
	rtest.OK(t, err)

	packData := p.Writer().(*bytes.Buffer).Bytes()
	return bufs, packData, p.Size()
}

func verifyBlobs(t testing.TB, bufs []Buf, k *crypto.Key, rd io.ReaderAt, packSize uint) {
	written := 0
	for _, buf := range bufs {
		written += len(buf.data)
	}
	// header length
	written += binary.Size(uint32(0))
	// header + header crypto
	headerSize := len(bufs) * (binary.Size(restic.BlobType(0)) + binary.Size(uint32(0)) + len(restic.ID{}))
	written += restic.CiphertextLength(headerSize)

	// check length
	rtest.Equals(t, uint(written), packSize)

	// read and parse it again
	entries, err := pack.List(k, rd, int64(packSize))
	rtest.OK(t, err)
	rtest.Equals(t, len(entries), len(bufs))

	var buf []byte
	for i, b := range bufs {
		e := entries[i]
		rtest.Equals(t, b.id, e.ID)

		if len(buf) < int(e.Length) {
			buf = make([]byte, int(e.Length))
		}
		buf = buf[:int(e.Length)]
		n, err := rd.ReadAt(buf, int64(e.Offset))
		rtest.OK(t, err)
		buf = buf[:n]

		rtest.Assert(t, bytes.Equal(b.data, buf),
			"data for blob %v doesn't match", i)
	}
}

func TestCreatePack(t *testing.T) {
	// create random keys
	k := crypto.NewRandomKey()

	bufs, packData, packSize := newPack(t, k, testLens)
	rtest.Equals(t, uint(len(packData)), packSize)
	verifyBlobs(t, bufs, k, bytes.NewReader(packData), packSize)
}

var blobTypeJSON = []struct {
	t   restic.BlobType
	res string
}{
	{restic.DataBlob, `"data"`},
	{restic.TreeBlob, `"tree"`},
}

func TestBlobTypeJSON(t *testing.T) {
	for _, test := range blobTypeJSON {
		// test serialize
		buf, err := json.Marshal(test.t)
		rtest.OK(t, err)
		rtest.Equals(t, test.res, string(buf))

		// test unserialize
		var v restic.BlobType
		err = json.Unmarshal([]byte(test.res), &v)
		rtest.OK(t, err)
		rtest.Equals(t, test.t, v)
	}
}

func TestUnpackReadSeeker(t *testing.T) {
	// create random keys
	k := crypto.NewRandomKey()

	bufs, packData, packSize := newPack(t, k, testLens)

	b := mem.New()
	id := restic.Hash(packData)

	handle := restic.Handle{Type: restic.PackFile, Name: id.String()}
	rtest.OK(t, b.Save(context.TODO(), handle, restic.NewByteReader(packData)))
	verifyBlobs(t, bufs, k, restic.ReaderAt(b, handle), packSize)
}

func TestShortPack(t *testing.T) {
	k := crypto.NewRandomKey()

	bufs, packData, packSize := newPack(t, k, []int{23})

	b := mem.New()
	id := restic.Hash(packData)

	handle := restic.Handle{Type: restic.PackFile, Name: id.String()}
	rtest.OK(t, b.Save(context.TODO(), handle, restic.NewByteReader(packData)))
	verifyBlobs(t, bufs, k, restic.ReaderAt(b, handle), packSize)
}
