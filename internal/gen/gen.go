// Copyright (c) 2026 the go-icons authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package gen turns the two upstream sources into the files twemoji embeds:
// the Twemoji SVG assets into twemoji.bin, markdown-it-emoji's shortcode
// table into shortcodes.txt, and Unicode's emoji-sequences.txt into the list
// of text-presentation characters, textdefault.txt.
package gen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-icons/twemoji/internal/pack"
)

// maxSVG bounds one asset: Twemoji's largest are a few kilobytes, so a file
// beyond this is not what the generator was told it is.
const maxSVG = 256 << 10

// Assets reads every .svg under dir (a Twemoji checkout's assets/svg).
func Assets(dir string) ([]pack.Entry, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.svg"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .svg files in %s", dir)
	}
	out := make([]pack.Entry, 0, len(files))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		if len(b) > maxSVG {
			return nil, fmt.Errorf("%s: %d bytes", f, len(b))
		}
		out = append(out, pack.Entry{Key: strings.TrimSuffix(filepath.Base(f), ".svg"), Data: b})
	}
	return out, nil
}

var defLine = regexp.MustCompile(`^\s*("(?:[^"\\]|\\.)*")\s*:\s*("(?:[^"\\]|\\.)*")\s*,?\s*$`)

// Shortcodes reads markdown-it-emoji's lib/data/full.mjs: one
// `"name": "sequence",` per line. It returns the table as name TAB sequence
// lines, sorted by name.
func Shortcodes(path string) ([]byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	table := map[string]string{}
	for _, line := range strings.Split(string(src), "\n") {
		m := defLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var name, seq string
		if err := json.Unmarshal([]byte(m[1]), &name); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(m[2]), &seq); err != nil {
			return nil, err
		}
		if name == "" || seq == "" || strings.ContainsAny(name+seq, "\t\n:") {
			return nil, fmt.Errorf("%s: unusable definition %q", path, line)
		}
		if _, dup := table[name]; dup {
			return nil, fmt.Errorf("%s: %q defined twice", path, name)
		}
		table[name] = seq
	}
	if len(table) == 0 {
		return nil, fmt.Errorf("%s: no definitions", path)
	}
	names := make([]string, 0, len(table))
	for n := range table {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n + "\t" + table[n] + "\n")
	}
	return []byte(b.String()), nil
}

var basicFE0F = regexp.MustCompile(`^([0-9A-Fa-f]{4,6}) FE0F\s*;\s*Basic_Emoji\b`)

// TextDefault reads the Unicode file emoji-sequences.txt and returns the
// code points that are emoji only when followed by U+FE0F (their default
// presentation is text: ©, ☺, ✔, ▶, ...), as one lower-case hexadecimal
// number per line, sorted. They are the Basic_Emoji lines that spell out
// "XXXX FE0F".
func TextDefault(path string) ([]byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cps []int
	for _, line := range strings.Split(string(src), "\n") {
		if m := basicFE0F.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseInt(m[1], 16, 32) // 4 to 6 hex digits, by the pattern
			cps = append(cps, int(v))
		}
	}
	if len(cps) == 0 {
		return nil, fmt.Errorf("%s: no Basic_Emoji lines with U+FE0F", path)
	}
	sort.Ints(cps)
	var b strings.Builder
	for _, c := range cps {
		b.WriteString(strconv.FormatInt(int64(c), 16) + "\n")
	}
	return []byte(b.String()), nil
}

// Build writes twemoji.bin, shortcodes.txt and textdefault.txt into out.
func Build(assetsDir, fullMJS, emojiSequences, out string) error {
	entries, err := Assets(assetsDir)
	if err != nil {
		return err
	}
	bin, err := pack.Encode(entries)
	if err != nil {
		return err
	}
	short, err := Shortcodes(fullMJS)
	if err != nil {
		return err
	}
	text, err := TextDefault(emojiSequences)
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{"twemoji.bin": bin, "shortcodes.txt": short, "textdefault.txt": text} {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
