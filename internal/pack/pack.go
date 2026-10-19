// Copyright (c) 2026 the go-icons authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package pack is the archive format of the embedded SVGs: documents sorted
// by key, cut into blocks of about BlockSize bytes, each block deflated on
// its own. Neighbouring keys are variants of one emoji (skin tones, genders),
// which share most of their markup, so a block compresses far better than the
// files would one by one; and only the block that holds a wanted document is
// ever inflated.
//
// Layout, little endian:
//
//	"TWM1" nBlocks:u32 nEntries:u32
//	nBlocks  × { compressedLen:u32 rawLen:u32 }
//	nEntries × { keyLen:u8 key block:u32 off:u32 len:u32 }
//	the compressed blocks, back to back
package pack

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// BlockSize is the raw size at which a block is closed.
const BlockSize = 48 << 10

// MaxBlock bounds what a block may inflate to: the reader refuses a block
// header that claims more, so a damaged archive cannot ask for gigabytes.
const MaxBlock = 4 << 20

const magic = "TWM1"

// Entry is one document.
type Entry struct {
	Key  string
	Data []byte
}

// Encode builds an archive. Keys must be non-empty, at most 255 bytes and
// distinct.
func Encode(entries []Entry) ([]byte, error) {
	es := append([]Entry(nil), entries...)
	sort.Slice(es, func(i, j int) bool { return es[i].Key < es[j].Key })

	type slot struct {
		key    string
		block  int
		off, n int
	}
	var (
		slots  []slot
		blocks [][2]int // compressed length, raw length
		comp   [][]byte
		raw    bytes.Buffer
	)
	flush := func() {
		if raw.Len() == 0 {
			return
		}
		// Writing to a bytes.Buffer, at a valid level, cannot fail.
		var zb bytes.Buffer
		zw, _ := flate.NewWriter(&zb, flate.BestCompression)
		_, _ = zw.Write(raw.Bytes())
		_ = zw.Close()
		blocks = append(blocks, [2]int{zb.Len(), raw.Len()})
		comp = append(comp, zb.Bytes())
		raw.Reset()
	}
	for i, e := range es {
		switch {
		case e.Key == "" || len(e.Key) > 255:
			return nil, fmt.Errorf("pack: key %q: length must be 1..255", e.Key)
		case i > 0 && es[i-1].Key == e.Key:
			return nil, fmt.Errorf("pack: duplicate key %q", e.Key)
		case len(e.Data) > MaxBlock:
			return nil, fmt.Errorf("pack: %q: %d bytes exceed the block bound", e.Key, len(e.Data))
		}
		if raw.Len() > 0 && raw.Len()+len(e.Data) > BlockSize {
			flush()
		}
		slots = append(slots, slot{key: e.Key, block: len(blocks), off: raw.Len(), n: len(e.Data)})
		raw.Write(e.Data)
	}
	flush()

	var out bytes.Buffer
	out.WriteString(magic)
	put := func(v int) { out.Write(binary.LittleEndian.AppendUint32(nil, uint32(v))) }
	put(len(blocks))
	put(len(slots))
	for _, b := range blocks {
		put(b[0])
		put(b[1])
	}
	for _, s := range slots {
		out.WriteByte(byte(len(s.key)))
		out.WriteString(s.key)
		put(s.block)
		put(s.off)
		put(s.n)
	}
	for _, c := range comp {
		out.Write(c)
	}
	return out.Bytes(), nil
}

type entry struct {
	key           string
	block, off, n uint32
}

type block struct {
	off, clen, rlen uint32
}

// Archive reads an encoded archive. It is safe for concurrent use.
type Archive struct {
	data    string
	blocks  []block
	entries []entry

	mu    sync.Mutex
	cache map[uint32]string
}

// cacheBlocks is how many inflated blocks an Archive keeps.
const cacheBlocks = 8

// Open validates the header and the tables of data. Keys and documents are
// returned as substrings of data (keys) or of a freshly inflated block
// (documents): nothing else is copied at Open.
func Open(data string) (*Archive, error) {
	if len(data) < 12 || data[:4] != magic {
		return nil, errors.New("pack: not an archive")
	}
	u32 := func(p int) uint32 {
		return uint32(data[p]) | uint32(data[p+1])<<8 | uint32(data[p+2])<<16 | uint32(data[p+3])<<24
	}
	nb, ne := int(u32(4)), int(u32(8))
	// Each table row is at least 8 bytes: a larger count cannot fit.
	if nb > len(data)/8 || ne > len(data)/8 {
		return nil, errors.New("pack: tables larger than the archive")
	}
	a := &Archive{data: data, blocks: make([]block, nb), entries: make([]entry, ne), cache: map[uint32]string{}}
	p := 12
	if p+8*nb > len(data) {
		return nil, errors.New("pack: truncated block table")
	}
	var total uint64
	for i := range a.blocks {
		a.blocks[i] = block{clen: u32(p), rlen: u32(p + 4)}
		p += 8
		if a.blocks[i].rlen > MaxBlock {
			return nil, fmt.Errorf("pack: block %d claims %d bytes", i, a.blocks[i].rlen)
		}
		total += uint64(a.blocks[i].clen)
	}
	for i := range a.entries {
		if p >= len(data) {
			return nil, errors.New("pack: truncated entry table")
		}
		kl := int(data[p])
		p++
		if kl == 0 || p+kl+12 > len(data) {
			return nil, errors.New("pack: truncated entry table")
		}
		e := entry{key: data[p : p+kl]}
		p += kl
		e.block, e.off, e.n = u32(p), u32(p+4), u32(p+8)
		p += 12
		if uint64(e.block) >= uint64(nb) || uint64(e.off)+uint64(e.n) > uint64(a.blocks[e.block].rlen) {
			return nil, fmt.Errorf("pack: entry %q outside its block", e.key)
		}
		if i > 0 && a.entries[i-1].key >= e.key {
			return nil, fmt.Errorf("pack: entry %q out of order", e.key)
		}
		a.entries[i] = e
	}
	if uint64(len(data)-p) != total {
		return nil, errors.New("pack: payload length does not match the block table")
	}
	for i := range a.blocks {
		a.blocks[i].off = uint32(p)
		p += int(a.blocks[i].clen)
	}
	return a, nil
}

// Len is the number of documents.
func (a *Archive) Len() int { return len(a.entries) }

// Key returns the i-th key, in sorted order.
func (a *Archive) Key(i int) string { return a.entries[i].key }

// Index returns the position of key, or -1.
func (a *Archive) Index(key string) int {
	i := sort.Search(len(a.entries), func(i int) bool { return a.entries[i].key >= key })
	if i < len(a.entries) && a.entries[i].key == key {
		return i
	}
	return -1
}

// Get returns the i-th document, inflating its block if no cached copy holds
// it.
func (a *Archive) Get(i int) (string, error) {
	e := a.entries[i]
	s, err := a.inflate(e.block)
	if err != nil {
		return "", err
	}
	return s[e.off : e.off+e.n], nil
}

func (a *Archive) inflate(i uint32) (string, error) {
	a.mu.Lock()
	s, ok := a.cache[i]
	a.mu.Unlock()
	if ok {
		return s, nil
	}
	b := a.blocks[i]
	zr := flate.NewReader(strings.NewReader(a.data[b.off : b.off+b.clen]))
	buf := make([]byte, b.rlen)
	if _, err := io.ReadFull(zr, buf); err != nil {
		return "", fmt.Errorf("pack: block %d: %w", i, err)
	}
	// Exactly rlen bytes: anything more is a lie in the header.
	if n, _ := zr.Read(make([]byte, 1)); n != 0 {
		return "", fmt.Errorf("pack: block %d inflates past its recorded length", i)
	}
	s = string(buf)
	a.mu.Lock()
	if len(a.cache) >= cacheBlocks {
		clear(a.cache)
	}
	a.cache[i] = s
	a.mu.Unlock()
	return s, nil
}
