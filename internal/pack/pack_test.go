package pack

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func sample(n int) []Entry {
	var es []Entry
	for i := range n {
		// Different keys, bodies big enough to span several blocks.
		es = append(es, Entry{
			Key:  fmt.Sprintf("k%04d", n-i), // unsorted on purpose
			Data: []byte(strings.Repeat(fmt.Sprintf("<svg id='%d'/>", i), 400)),
		})
	}
	return es
}

func TestRoundTrip(t *testing.T) {
	es := sample(40)
	enc, err := Encode(es)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Open(string(enc))
	if err != nil {
		t.Fatal(err)
	}
	if a.Len() != len(es) || len(a.blocks) < 3 {
		t.Fatalf("len %d blocks %d", a.Len(), len(a.blocks))
	}
	for _, e := range es {
		i := a.Index(e.Key)
		if i < 0 || a.Key(i) != e.Key {
			t.Fatalf("Index(%q) = %d", e.Key, i)
		}
		got, err := a.Get(i)
		if err != nil || got != string(e.Data) {
			t.Fatalf("Get(%q) = %.20q, %v", e.Key, got, err)
		}
	}
	for _, k := range []string{"", "a", "k0000", "k9999", "zzz"} {
		if a.Index(k) != -1 {
			t.Errorf("Index(%q) found", k)
		}
	}
}

func TestEmptyArchive(t *testing.T) {
	enc, err := Encode(nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Open(string(enc))
	if err != nil || a.Len() != 0 || a.Index("x") != -1 {
		t.Fatalf("%v %v", a, err)
	}
}

func TestCacheEviction(t *testing.T) {
	enc, _ := Encode(sample(150))
	a, err := Open(string(enc))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.blocks) <= cacheBlocks {
		t.Fatalf("only %d blocks", len(a.blocks))
	}
	for i := range a.Len() {
		if _, err := a.Get(i); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.cache) > cacheBlocks {
		t.Errorf("cache holds %d blocks", len(a.cache))
	}
}

func TestConcurrent(t *testing.T) {
	enc, _ := Encode(sample(30))
	a, _ := Open(string(enc))
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range a.Len() {
				if _, err := a.Get((i + g) % a.Len()); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestEncodeErrors(t *testing.T) {
	cases := map[string][]Entry{
		"empty key": {{Key: "", Data: []byte("x")}},
		"long key":  {{Key: strings.Repeat("a", 256), Data: []byte("x")}},
		"duplicate": {{Key: "a"}, {Key: "a"}},
		"too big":   {{Key: "a", Data: make([]byte, MaxBlock+1)}},
	}
	for name, es := range cases {
		if _, err := Encode(es); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// TestTruncation: every prefix of a valid archive is refused, none panics.
func TestTruncation(t *testing.T) {
	enc, _ := Encode(sample(5))
	for n := range len(enc) {
		if _, err := Open(string(enc[:n])); err == nil {
			t.Fatalf("prefix of %d bytes opened", n)
		}
	}
}

func put32(b []byte, p int, v uint32) {
	b[p], b[p+1], b[p+2], b[p+3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}

func TestOpenRefusals(t *testing.T) {
	good, _ := Encode([]Entry{{Key: "a", Data: []byte("<svg/>")}, {Key: "b", Data: []byte("<svg/>")}})
	// Offsets: header 12, one block row (8) at 12, entries from 20:
	// a: len(1) "a"(1) block(4) off(4) n(4) = 14 bytes; b follows at 34.
	mut := func(f func(b []byte)) string {
		b := bytes.Clone(good)
		f(b)
		return string(b)
	}
	cases := map[string]string{
		"magic":           mut(func(b []byte) { b[0] = 'X' }),
		"huge blocks":     mut(func(b []byte) { put32(b, 4, 1<<30) }),
		"huge entries":    mut(func(b []byte) { put32(b, 8, 1<<30) }),
		"block too large": mut(func(b []byte) { put32(b, 16, MaxBlock+1) }),
		"zero key length": mut(func(b []byte) { b[20] = 0 }),
		"block range":     mut(func(b []byte) { b[22] = 9 }),
		"block table":     mut(func(b []byte) { put32(b, 4, uint32(len(b)/8)) }),
		"entry range":     mut(func(b []byte) { put32(b, 26, 1000) }),
		"order":           mut(func(b []byte) { b[21], b[35] = 'b', 'a' }),
		"payload":         string(good) + "x",
	}
	for name, data := range cases {
		if _, err := Open(data); err == nil {
			t.Errorf("%s: opened", name)
		}
	}
}

func TestInflateRefusals(t *testing.T) {
	good, _ := Encode([]Entry{{Key: "a", Data: []byte(strings.Repeat("<svg/>", 50))}})
	// The block row is at 12: compressedLen, rawLen.
	a, err := Open(string(good))
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt the compressed bytes.
	bad := bytes.Clone(good)
	clen := int(good[12]) | int(good[13])<<8 | int(good[14])<<16
	bad[len(bad)-clen] = 0x07 // BFINAL with BTYPE 11, a reserved type: invalid in every version
	b, err := Open(string(bad))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Get(0); err == nil {
		t.Error("corrupt block inflated")
	}
	// A header that claims fewer bytes than the block holds.
	short := bytes.Clone(good)
	put32(short, 16, 100) // rawLen 100 < 300
	put32(short, 30, 10)  // the entry, shrunk to fit
	c, err := Open(string(short))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(0); err == nil {
		t.Error("block longer than its header inflated")
	}
	if got, err := a.Get(0); err != nil || len(got) != 300 {
		t.Errorf("good archive: %d bytes, %v", len(got), err)
	}
}
