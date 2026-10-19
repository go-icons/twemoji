// Copyright (c) 2026 the go-icons authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package twemoji serves the Twemoji emoji graphics (jdecked/twemoji, the
// community continuation of Twitter's set; graphics CC-BY 4.0 — see
// TWEMOJI-LICENSE-GRAPHICS) as SVG documents keyed by emoji sequence, with
// the :shortcode: table of markdown-it-emoji (MIT — see
// MARKDOWN-IT-EMOJI-LICENSE).
//
// It is a DATA package: it embeds the SVGs, compressed, and hands one back;
// it draws nothing. The archive is inflated lazily, a block at a time, and
// the lookup tables are built on first use, so importing the package costs a
// few kilobytes of address space and no start-up time.
//
// # Attribution
//
// The graphics are licensed CC-BY 4.0: whoever shows them must credit
// "Twemoji, Copyright 2020 Twitter, Inc and other contributors (CC-BY 4.0),
// continued by the jdecked/twemoji project". Attribution is the one condition
// this package cannot meet for its users.
//
// # Names
//
// An emoji's STEM is the file name Twemoji gives its graphic: the code points
// in lower-case hexadecimal joined with "-", with U+FE0F (the emoji
// presentation selector) dropped unless the sequence has a ZWJ: 😄 is
// "1f604", 🇫🇷 is "1f1eb-1f1f7", 👩‍💻 is "1f469-200d-1f4bb", 1️⃣ is "31-20e3".
// Lookups here ignore U+FE0F altogether, so a sequence finds its graphic
// however the text qualified it.
package twemoji

import (
	_ "embed" // the data files
	"errors"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-icons/twemoji/internal/pack"
)

// Version is the Twemoji release the embedded graphics come from, and
// ShortcodesVersion the markdown-it-emoji release of the :shortcode: table.
const (
	Version           = "17.0.3"
	ShortcodesVersion = "3.0.0"
)

// Attribution is the credit CC-BY 4.0 asks for.
const Attribution = "Twemoji, Copyright 2020 Twitter, Inc and other contributors (CC-BY 4.0), continued by the jdecked/twemoji project"

//go:embed twemoji.bin
var archiveData string

//go:embed shortcodes.txt
var shortcodeData string

const (
	zwj   = 0x200D
	vs16  = 0xFE0F // emoji presentation selector
	vs15  = 0xFE0E // text presentation selector
	heart = 0x2764 // ❤, the text-default character emoji-regex takes bare
)

//go:embed textdefault.txt
var textDefaultData string

var (
	textOnce sync.Once
	textSet  map[rune]struct{}
)

// TextDefault reports whether r is an emoji whose default presentation is
// text: a plain glyph unless U+FE0F follows it (© ™ ↔ ☺ ✔ ▶ and about 200
// more, from Unicode's emoji-sequences.txt).
func TextDefault(r rune) bool {
	textOnce.Do(func() {
		textSet = map[rune]struct{}{}
		for _, line := range strings.Fields(textDefaultData) {
			if v, err := strconv.ParseUint(line, 16, 32); err == nil {
				textSet[rune(v)] = struct{}{}
			}
		}
	})
	_, ok := textSet[r]
	return ok
}

// maxRunes bounds the longest sequence looked at from one position: the
// longest in Twemoji (a family with skin tones) has 11 code points.
const maxRunes = 16

var (
	once    sync.Once
	loadErr error
	arc     *pack.Archive
	byNorm  map[string]int32 // sequence without U+FE0F -> archive position
	firsts  map[rune]struct{}
)

func load() {
	once.Do(func() {
		a, err := pack.Open(archiveData)
		if err != nil {
			loadErr = err
			return
		}
		m := make(map[string]int32, a.Len())
		f := map[rune]struct{}{}
		for i := range a.Len() {
			rs, err := parseStem(a.Key(i))
			if err != nil {
				loadErr = err
				return
			}
			n := normalize(rs)
			if _, dup := m[n]; !dup {
				m[n] = int32(i)
			}
			f[rs[0]] = struct{}{}
		}
		arc, byNorm, firsts = a, m, f
	})
}

// Err reports why the embedded data could not be read: nil unless the
// package was built from a damaged archive, which its tests rule out.
func Err() error {
	load()
	return loadErr
}

func parseStem(stem string) ([]rune, error) {
	var rs []rune
	for _, h := range strings.Split(stem, "-") {
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil || !utf8.ValidRune(rune(v)) {
			return nil, errors.New("twemoji: bad asset name " + strconv.Quote(stem))
		}
		rs = append(rs, rune(v))
	}
	return rs, nil
}

// normalize is the lookup key of a sequence: the sequence without U+FE0F.
func normalize(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		if r != vs16 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Stem is the name Twemoji gives the graphic of seq, computed the way
// twemoji.js does: lower-case hexadecimal code points joined with "-",
// U+FE0F dropped unless seq contains a ZWJ. It does not look at the
// embedded set; see Lookup.
func Stem(seq string) string {
	hasZWJ := strings.ContainsRune(seq, zwj)
	var b strings.Builder
	for _, r := range seq {
		if r == vs16 && !hasZWJ {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('-')
		}
		b.WriteString(strconv.FormatInt(int64(r), 16))
	}
	return b.String()
}

// Lookup returns the stem of the embedded graphic for seq, ignoring U+FE0F,
// or "" when the set has none.
func Lookup(seq string) string {
	load()
	if arc == nil {
		return ""
	}
	i, ok := byNorm[normalize([]rune(seq))]
	if !ok {
		return ""
	}
	return arc.Key(int(i))
}

// SVG returns the SVG document of the graphic for seq (a sequence in any
// qualification: with or without U+FE0F), and whether the set has one.
func SVG(seq string) (string, bool) {
	load()
	if arc == nil {
		return "", false
	}
	i, ok := byNorm[normalize([]rune(seq))]
	if !ok {
		return "", false
	}
	svg, err := arc.Get(int(i))
	return svg, err == nil
}

// SVGByStem is SVG for a stem as Stem or Lookup return it.
func SVGByStem(stem string) (string, bool) {
	load()
	if arc == nil {
		return "", false
	}
	i := arc.Index(stem)
	if i < 0 {
		return "", false
	}
	svg, err := arc.Get(i)
	return svg, err == nil
}

// Len is how many graphics the set holds.
func Len() int {
	load()
	if arc == nil {
		return 0
	}
	return arc.Len()
}

// Stems lists the stem of every graphic, sorted.
func Stems() []string {
	load()
	if arc == nil {
		return nil
	}
	out := make([]string, arc.Len())
	for i := range out {
		out[i] = arc.Key(i)
	}
	return out
}

// Cut returns the byte length of the emoji at the start of s, or 0 when s
// does not start with one. It reads text the way Marp Core does (the
// emoji-regex it matches with, then Twemoji's parser):
//
//   - the match is the longest sequence the set has a graphic for, with or
//     without U+FE0F inside it;
//   - a character whose default presentation is text (©, ™, ☺, ✔, ▶: see
//     TextDefault) is an emoji only when U+FE0F follows it — except ❤
//     (U+2764), which is an emoji bare;
//   - a U+FE0F after a character that is already an emoji by default (😄) is
//     not part of the match; after a text-default one it is (Marp Core also
//     takes it after about thirty BMP emoji such as ⚽ ☕ ♈; the difference
//     is an invisible U+FE0F left in the text or not);
//   - a match followed by U+FE0E (text presentation selector) is not one;
//   - a lone regional indicator is not a flag, a bare digit, # or * is not a
//     keycap (they are with U+20E3), and nothing in the private use area is
//     an emoji.
func Cut(s string) int {
	load()
	if arc == nil || s == "" {
		return 0
	}
	first, _ := utf8.DecodeRuneInString(s)
	if first < 0x80 {
		// Only keycaps start in ASCII: # * 0-9, then FE0F? 20E3.
		if !(first == '#' || first == '*' || (first >= '0' && first <= '9')) || len(s) < 4 {
			return 0
		}
	}
	if _, ok := firsts[first]; !ok {
		return 0
	}
	var (
		rs   [maxRunes + 1]rune
		ends [maxRunes + 1]int // ends[i]: byte offset just after rs[i]
		n    int
	)
	for off := 0; off < len(s) && n < len(rs); {
		r, w := utf8.DecodeRuneInString(s[off:])
		if n > 0 && r < 0xA9 || r == utf8.RuneError {
			break
		}
		off += w
		rs[n], ends[n] = r, off
		n++
	}
	for k := min(n, maxRunes); k > 0; k-- {
		if _, ok := byNorm[normalize(rs[:k])]; !ok {
			continue
		}
		if (rs[0] >= 0x1F1E6 && rs[0] <= 0x1F1FF || rs[0] >= 0xE000 && rs[0] <= 0xF8FF) && onlyVS16(rs[1:k]) {
			continue // one regional indicator is no flag; private use is no emoji
		}
		if TextDefault(rs[0]) && rs[0] != heart && (k < 2 || rs[1] != vs16) {
			continue
		}
		end := ends[k-1]
		// A U+FE0F after a character that is an emoji by default (😄) is
		// left out; after a text-default one it is part of the match.
		if k >= 2 && rs[k-1] == vs16 && !TextDefault(rs[k-2]) {
			end = ends[k-2]
		}
		if end < len(s) {
			if r, _ := utf8.DecodeRuneInString(s[end:]); r == vs15 {
				continue
			}
		}
		return end
	}
	return 0
}

func onlyVS16(rs []rune) bool {
	for _, r := range rs {
		if r != vs16 {
			return false
		}
	}
	return true
}

// Find returns the byte range of the first emoji of s at or after from, or
// start == -1.
func Find(s string, from int) (start, end int) {
	for i := from; i < len(s); {
		if n := Cut(s[i:]); n > 0 {
			return i, i + n
		}
		_, w := utf8.DecodeRuneInString(s[i:])
		i += w
	}
	return -1, -1
}

var (
	shortOnce sync.Once
	shortList []string // "name\tsequence" lines, sorted by name
)

func shorts() []string {
	shortOnce.Do(func() {
		shortList = strings.Split(strings.TrimSuffix(shortcodeData, "\n"), "\n")
	})
	return shortList
}

// Shortcode returns the sequence of the :name: shortcode (name without the
// colons: "smile", "+1", "100"), and whether it is defined.
func Shortcode(name string) (string, bool) {
	l := shorts()
	lo, hi := 0, len(l)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		n, _, _ := strings.Cut(l[m], "\t")
		if n < name {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(l) {
		if n, seq, _ := strings.Cut(l[lo], "\t"); n == name {
			return seq, true
		}
	}
	return "", false
}

// Shortcodes lists every shortcode name, sorted.
func Shortcodes() []string {
	l := shorts()
	out := make([]string, len(l))
	for i, line := range l {
		out[i], _, _ = strings.Cut(line, "\t")
	}
	return out
}

// Replace is the :shortcode: pass of markdown-it-emoji over text: each
// :name: that names a shortcode becomes f(name, sequence). Matches do not
// overlap; ":a:smile:" yields the smile, since ":a:" names nothing. Text
// without a colon is returned as is.
func Replace(s string, f func(name, seq string) string) string {
	if !strings.Contains(s, ":") {
		return s
	}
	var b strings.Builder
	last := 0
	for i := 0; i < len(s); {
		if s[i] != ':' {
			i++
			continue
		}
		j := strings.IndexByte(s[i+1:], ':')
		if j < 0 {
			break
		}
		j += i + 1
		if seq, ok := Shortcode(s[i+1 : j]); ok {
			b.WriteString(s[last:i])
			b.WriteString(f(s[i+1:j], seq))
			last, i = j+1, j+1
			continue
		}
		i++
	}
	if last == 0 {
		return s
	}
	b.WriteString(s[last:])
	return b.String()
}
