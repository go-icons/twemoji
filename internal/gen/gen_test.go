package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-icons/twemoji/internal/pack"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const fullMJS = `// Generated, don't edit
export default {
  "100": "💯",
  "smile": "😄",
  "heart": "\u2764\ufe0f",
  "+1": "👍"
}
`

func TestBuild(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "svg", "1f604.svg"), "<svg>a</svg>")
	write(t, filepath.Join(dir, "svg", "1f44d.svg"), "<svg>b</svg>")
	write(t, filepath.Join(dir, "svg", "readme.txt"), "ignored")
	write(t, filepath.Join(dir, "full.mjs"), fullMJS)
	write(t, filepath.Join(dir, "seq.txt"), seqTXT)
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Build(filepath.Join(dir, "svg"), filepath.Join(dir, "full.mjs"), filepath.Join(dir, "seq.txt"), out); err != nil {
		t.Fatal(err)
	}
	bin, _ := os.ReadFile(filepath.Join(out, "twemoji.bin"))
	a, err := pack.Open(string(bin))
	if err != nil || a.Len() != 2 {
		t.Fatalf("%v %v", a, err)
	}
	if s, _ := a.Get(a.Index("1f604")); s != "<svg>a</svg>" {
		t.Errorf("got %q", s)
	}
	short, _ := os.ReadFile(filepath.Join(out, "shortcodes.txt"))
	want := "+1\t👍\n100\t💯\nheart\t❤️\nsmile\t😄\n"
	if string(short) != want {
		t.Errorf("shortcodes %q, want %q", short, want)
	}
	text, _ := os.ReadFile(filepath.Join(out, "textdefault.txt"))
	if string(text) != "a9\n263a\n2764\n" {
		t.Errorf("textdefault %q", text)
	}
}

const seqTXT = `# Basic_Emoji
231A..231B    ; Basic_Emoji                  ; watch..hourglass done                                          # E0.6   [2] (⌚..⌛)
263A FE0F     ; Basic_Emoji                  ; smiling face
2764 FE0F     ; Basic_Emoji                  ; red heart
00A9 FE0F     ; Basic_Emoji                  ; copyright
0023 FE0F 20E3; Emoji_Keycap_Sequence        ; keycap: #
`

func TestTextDefaultErrors(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "none.txt"), "# nothing\n")
	if _, err := TextDefault(filepath.Join(dir, "none.txt")); err == nil {
		t.Error("no lines")
	}
	if _, err := TextDefault(filepath.Join(dir, "absent.txt")); err == nil {
		t.Error("absent")
	}
}

func TestBuildErrors(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "svg", "a.svg"), "<svg/>")
	write(t, filepath.Join(dir, "full.mjs"), fullMJS)
	write(t, filepath.Join(dir, "seq.txt"), seqTXT)
	svg, mjs, seq := filepath.Join(dir, "svg"), filepath.Join(dir, "full.mjs"), filepath.Join(dir, "seq.txt")

	if err := Build(filepath.Join(dir, "none"), mjs, seq, dir); err == nil {
		t.Error("no assets dir")
	}
	if err := Build(svg, filepath.Join(dir, "none.mjs"), seq, dir); err == nil {
		t.Error("no shortcode file")
	}
	if err := Build(svg, mjs, filepath.Join(dir, "none.txt"), dir); err == nil {
		t.Error("no sequences file")
	}
	if err := Build(svg, mjs, seq, filepath.Join(dir, "missing", "out")); err == nil {
		t.Error("unwritable output")
	}
	// Output directories that refuse one of the files: it is a directory.
	for _, name := range []string{"twemoji.bin", "shortcodes.txt", "textdefault.txt"} {
		out := filepath.Join(dir, "out-"+name)
		if err := os.MkdirAll(filepath.Join(out, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Build(svg, mjs, seq, out); err == nil {
			t.Errorf("%s is a directory", name)
		}
	}
	// A key the archive refuses: ".svg" has an empty stem.
	empty := t.TempDir()
	write(t, filepath.Join(empty, ".svg"), "<svg/>")
	if err := Build(empty, mjs, seq, dir); err == nil {
		t.Error("empty key")
	}
}

func TestAssetsErrors(t *testing.T) {
	if _, err := Assets("[bad"); err == nil {
		t.Error("bad glob pattern")
	}
	dir := t.TempDir()
	if _, err := Assets(dir); err == nil {
		t.Error("empty dir")
	}
	// A .svg that cannot be read: a directory.
	if err := os.Mkdir(filepath.Join(dir, "x.svg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Assets(dir); err == nil {
		t.Error("unreadable asset")
	}
	big := t.TempDir()
	write(t, filepath.Join(big, "big.svg"), strings.Repeat("x", maxSVG+1))
	if _, err := Assets(big); err == nil {
		t.Error("oversized asset")
	}
}

func TestShortcodesErrors(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"empty":     "export default {\n}\n",
		"colon":     "  \"a:b\": \"x\",\n",
		"empty seq": "  \"a\": \"\",\n",
		"dup":       "  \"a\": \"x\",\n  \"a\": \"y\",\n",
		"bad name":  "  \"\\x\": \"y\",\n",
		"bad value": "  \"a\": \"\\x\",\n",
	} {
		p := filepath.Join(dir, name+".mjs")
		write(t, p, body)
		if _, err := Shortcodes(p); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if _, err := Shortcodes(filepath.Join(dir, "absent")); err == nil {
		t.Error("absent file")
	}
}
