package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	svgDir := filepath.Join(dir, "tw", "assets", "svg")
	mdDir := filepath.Join(dir, "md", "lib", "data")
	for _, d := range []string{svgDir, mdDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(svgDir, "1f604.svg"), []byte("<svg/>"), 0o644)
	_ = os.WriteFile(filepath.Join(mdDir, "full.mjs"), []byte("  \"smile\": \"😄\",\n"), 0o644)
	seq := filepath.Join(dir, "seq.txt")
	_ = os.WriteFile(seq, []byte("263A FE0F ; Basic_Emoji ; smiling face\n"), 0o644)

	var out bytes.Buffer
	cmd := newRoot(&out)
	cmd.SetArgs([]string{"--twemoji", filepath.Join(dir, "tw"), "--markdown-it-emoji", filepath.Join(dir, "md"), "--emoji-sequences", seq, "--out", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "wrote") {
		t.Errorf("output %q", out.String())
	}
	for _, f := range []string{"twemoji.bin", "shortcodes.txt", "textdefault.txt"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Error(err)
		}
	}
}

func TestRunErrors(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--twemoji", "x"},
		{"--twemoji", "/nonexistent", "--markdown-it-emoji", "/nonexistent", "--emoji-sequences", "/nonexistent"},
		{"extra", "--twemoji", "x", "--markdown-it-emoji", "y", "--emoji-sequences", "z"},
	} {
		cmd := newRoot(&bytes.Buffer{})
		cmd.SetArgs(args)
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}
