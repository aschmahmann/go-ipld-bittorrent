package testing

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	ipldbencode "github.com/aschmahmann/go-ipld-bittorrent/bencode"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"io/ioutil"
	"testing"
)

func TestLoadFile(t *testing.T) {
	fb, err := ioutil.ReadFile("Koala.jpg.torrent")
	//file, err := os.Open()
	if err != nil {
		t.Fatal(err)
	}
	file := bytes.NewReader(fb)
	nb := basicnode.Prototype.Any.NewBuilder()
	if err := ipldbencode.Decode(nb, file); err != nil {
		t.Fatal(err)
	}

	buf := &bytes.Buffer{}
	node := nb.Build()
	if err := dagjson.Encode(node, buf); err != nil {
		t.Fatal(err)
	}

	t.Logf("output: %s", buf.String())

	// {"comment":"dynamic metainfo from client","created by":"go.torrent","creation date":1648770517,"info":{"length":273042,"name":"Koala.jpg","piece length":262144,"pieces":"\u0013\ufffdV\ufffd\u0010҈v\ufffdަ-FEr\ufffd\ufffdn\ufffd}O\u0002\ufffd\u0004\ufffd\ufffd\u0015\ufffd\u0016\ufffd\u001b\ufffd4\ufffdh\ufffd[\ufffd\ufffd\ufffd"},"url-list":[]}

	// Check infohash which is: 71c582c2dc2420cda8a3a648517c70ee690d0224
	infoDict, err := node.LookupByString("info")
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	if err := ipldbencode.Encode(infoDict, buf); err != nil {
		t.Fatal(err)
	}

	infohash := sha1.Sum(buf.Bytes())
	ihStr := hex.EncodeToString(infohash[:])
	if ihStr != "71c582c2dc2420cda8a3a648517c70ee690d0224" {
		t.Fatalf("wrong infohash: got %s", ihStr)
	}

	pieces, err := infoDict.LookupByString("pieces")
	if err != nil {
		t.Fatal(err)
	}

	piecesStr, err := pieces.AsString()
	if err != nil {
		t.Fatal(err)
	}

	dataFile, err := ioutil.ReadFile("Koala.jpg")
	if err != nil {
		t.Fatal(err)
	}

	const hashlen = 20
	chunkSizeNd, err := infoDict.LookupByString("piece length")
	if err != nil {
		t.Fatal(err)
	}

	chunkSize, err := chunkSizeNd.AsInt()
	if err != nil {
		t.Fatal(err)
	}
	
	for i, j :=0,0 ; i<len(piecesStr); i, j = i+hashlen, j+1 {
		start := j * int(chunkSize)
		end := start + int(chunkSize)
		if end > len(dataFile) {
			end = len(dataFile)
		}
		computedHash := sha1.Sum(dataFile[start:end])
		storedHash := []byte(piecesStr[i:i+hashlen])
		if !bytes.Equal(computedHash[:], storedHash) {
			t.Fatal("hashes not equal")
		}
		t.Logf("%x", computedHash)
	}
}
