package twemoji

import (
	"strings"
	"sync"
	"testing"
)

func TestDataLoads(t *testing.T) {
	if err := Err(); err != nil {
		t.Fatal(err)
	}
	if Len() < 3500 {
		t.Fatalf("only %d graphics", Len())
	}
	t.Logf("%d graphics, %d bytes embedded, %d shortcodes", Len(), len(archiveData), len(Shortcodes()))
}

// TestEveryStem: each stem names a document that is an SVG, parses back into
// a sequence, and is found again from it, however FE0F is placed.
func TestEveryStem(t *testing.T) {
	for _, stem := range Stems() {
		svg, ok := SVGByStem(stem)
		if !ok || !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(strings.TrimSpace(svg), "</svg>") {
			t.Fatalf("%s: %.40q %v", stem, svg, ok)
		}
		rs, err := parseStem(stem)
		if err != nil {
			t.Fatal(err)
		}
		if got := Lookup(string(rs)); got != stem {
			t.Fatalf("Lookup(%s) = %q", stem, got)
		}
	}
}

// TestCutEveryStem: written the way text qualifies it (U+FE0F after a
// text-default first character), every sequence the set holds is cut whole,
// except what is not an emoji alone.
func TestCutEveryStem(t *testing.T) {
	for _, stem := range Stems() {
		rs, _ := parseStem(stem)
		if len(rs) == 1 && (rs[0] >= 0x1F1E6 && rs[0] <= 0x1F1FF || rs[0] >= 0xE000 && rs[0] <= 0xF8FF) {
			if Cut(string(rs)) != 0 {
				t.Errorf("%s: a lone regional indicator or private use character was cut", stem)
			}
			continue
		}
		if TextDefault(rs[0]) && (len(rs) < 2 || rs[1] != vs16) {
			rs = append([]rune{rs[0], vs16}, rs[1:]...)
		}
		s := string(rs)
		if n := Cut(s + " x"); n != len(s) && n != len(s)-len("️") {
			t.Errorf("Cut(%s) = %d of %d", stem, n, len(s))
		}
	}
}

func TestTextDefault(t *testing.T) {
	for _, r := range "©®™↔☺✔▶" {
		if !TextDefault(r) {
			t.Errorf("%q is not text-default", r)
		}
	}
	for _, r := range "😄a🇫1" {
		if TextDefault(r) {
			t.Errorf("%q is text-default", r)
		}
	}
}

func TestStem(t *testing.T) {
	cases := map[string]string{
		"😄":       "1f604",
		"🇫🇷":      "1f1eb-1f1f7",
		"👩‍💻":     "1f469-200d-1f4bb",
		"1️⃣":     "31-20e3",
		"❤️":      "2764",
		"❤":       "2764",
		"🏳️‍🌈":    "1f3f3-fe0f-200d-1f308",
		"👨‍👩‍👧‍👦": "1f468-200d-1f469-200d-1f467-200d-1f466",
		"":        "",
		"️":       "",
		"🏴󠁧󠁢󠁥󠁮󠁧󠁿": "1f3f4-e0067-e0062-e0065-e006e-e0067-e007f",
	}
	for in, want := range cases {
		if got := Stem(in); got != want {
			t.Errorf("Stem(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLookupAndSVG(t *testing.T) {
	for in, want := range map[string]string{
		"😄":    "1f604",
		"🇫🇷":   "1f1eb-1f1f7",
		"👩‍💻":  "1f469-200d-1f4bb",
		"1️⃣":  "31-20e3",
		"1⃣":   "31-20e3", // without FE0F
		"❤️":   "2764",
		"☺":    "263a",
		"☺️":   "263a",
		"🏳‍🌈":  "1f3f3-fe0f-200d-1f308", // unqualified ZWJ sequence
		"🏳️‍🌈": "1f3f3-fe0f-200d-1f308",
		"👩🏽‍💻": "1f469-1f3fd-200d-1f4bb",
	} {
		if got := Lookup(in); got != want {
			t.Errorf("Lookup(%q) = %q, want %q", in, got, want)
		}
		if svg, ok := SVG(in); !ok || !strings.Contains(svg, "<svg") {
			t.Errorf("SVG(%q) = %.30q, %v", in, svg, ok)
		}
	}
	for _, in := range []string{"", "a", "12", "\U0010FFFF", "😄😄"} {
		if got := Lookup(in); got != "" {
			t.Errorf("Lookup(%q) = %q", in, got)
		}
		if _, ok := SVG(in); ok {
			t.Errorf("SVG(%q) found", in)
		}
	}
	if _, ok := SVGByStem("nope"); ok {
		t.Error("SVGByStem(nope) found")
	}
}

func TestCut(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"😄", 4},
		{"😄abc", 4},
		{"abc😄", 0},
		{"", 0},
		{"abc", 0},
		{"é", 0},
		{"1", 0},
		{"12", 0},
		{"#", 0},
		{"1️⃣x", 7},
		{"1⃣", 4},
		{"*️⃣", 7},
		{"#️⃣", 7},
		{"☺", 0}, // text by default: an emoji only with U+FE0F
		{"☺️", 6},
		{"☺︎", 0}, // text presentation selector: left as text
		{"😄︎", 0},
		{"😄️", 4}, // U+FE0F after an emoji by default is not part of it
		{"🇫️", 0},
		{"❤", 3}, // the one text-default character taken bare
		{"❤️", 6},
		{"©", 0},
		{"©️", 5},
		{"✔", 0},
		{"✔️", 6},
		{"☝🏽", 0}, // a text-default base still needs U+FE0F; the tone alone is cut next
		{"☝️🏽", 10},
		{"", 0},
		{"👩‍💻", 11},
		{"👩‍💻!", 11},
		{"👩‍", 4}, // a dangling ZWJ is not part of it
		{"🇫🇷", 8},
		{"🇫", 0}, // one regional indicator is no flag
		{"🇫🇷🇩🇪", 8},
		{"👍🏽", 8},
		{"👨‍👩‍👧‍👦", 25},
		{"\xff😄", 0},
		{"😄\xff", 4},
	}
	for _, c := range cases {
		if got := Cut(c.in); got != c.want {
			t.Errorf("Cut(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	// A very long run of joiners is bounded, not quadratic.
	if n := Cut("😄" + strings.Repeat("‍😄", 1000)); n == 0 {
		t.Error("long run: no match")
	}
}

func TestFind(t *testing.T) {
	s := "a😄b👩‍💻c ☺ ✔"
	i, j := Find(s, 0)
	if s[i:j] != "😄" {
		t.Fatalf("first: %q", s[i:j])
	}
	i, j = Find(s, j)
	if s[i:j] != "👩‍💻" {
		t.Fatalf("second: %q", s[i:j])
	}
	if i, j = Find(s, j); i != -1 || j != -1 {
		t.Fatalf("third: %d %d", i, j)
	}
	if i, _ = Find("é\xffz", 0); i != -1 {
		t.Fatalf("none: %d", i)
	}
}

func TestShortcode(t *testing.T) {
	for name, want := range map[string]string{
		"smile": "😄", "+1": "👍", "-1": "👎", "100": "💯", "1234": "🔢",
		"heart": "❤️", "fr": "🇫🇷", "zzz": "💤",
	} {
		if got, ok := Shortcode(name); !ok || got != want {
			t.Errorf("Shortcode(%q) = %q, %v; want %q", name, got, ok, want)
		}
	}
	for _, name := range []string{"", "nope", "smil", "smilee", "Smile", "zzzz"} {
		if got, ok := Shortcode(name); ok {
			t.Errorf("Shortcode(%q) = %q", name, got)
		}
	}
	names := Shortcodes()
	if len(names) < 1800 {
		t.Fatalf("%d shortcodes", len(names))
	}
	missing := 0
	for _, n := range names {
		seq, ok := Shortcode(n)
		if !ok {
			t.Fatalf("listed but not found: %q", n)
		}
		if Lookup(seq) == "" {
			missing++
			t.Logf("no graphic for :%s: %q", n, seq)
		}
	}
	if missing > 0 {
		t.Errorf("%d shortcodes without a graphic", missing)
	}
}

func TestReplace(t *testing.T) {
	f := func(name, seq string) string { return "[" + name + "=" + seq + "]" }
	for in, want := range map[string]string{
		"":                      "",
		"no colon":              "no colon",
		"a: b":                  "a: b",
		"12:30:45":              "12:30:45",
		":smile:":               "[smile=😄]",
		"x :smile: y":           "x [smile=😄] y",
		":smile::+1:":           "[smile=😄][+1=👍]",
		":a:smile:":             "[a=🅰️]smile:", // ":a:" is a shortcode too
		":zz_:smile:":           ":zz_[smile=😄]",
		":nope: :smile":         ":nope: :smile",
		"::":                    "::",
		":smile":                ":smile",
		"time 12:30 :smile:end": "time 12:30 [smile=😄]end",
	} {
		if got := Replace(in, f); got != want {
			t.Errorf("Replace(%q) = %q, want %q", in, got, want)
		}
	}
}

// reset swaps the embedded data and forgets what was loaded.
func reset(t *testing.T, archive string) {
	t.Helper()
	old := archiveData
	t.Cleanup(func() {
		archiveData = old
		once, arc, byNorm, firsts, loadErr = sync.Once{}, nil, nil, nil, nil
	})
	archiveData = archive
	once, arc, byNorm, firsts, loadErr = sync.Once{}, nil, nil, nil, nil
}

func TestDamagedData(t *testing.T) {
	reset(t, "not an archive")
	if Err() == nil {
		t.Fatal("no error")
	}
	if Len() != 0 || Stems() != nil || Lookup("😄") != "" || Cut("😄") != 0 || Cut("") != 0 {
		t.Error("a damaged set answered")
	}
	if _, ok := SVG("😄"); ok {
		t.Error("SVG answered")
	}
	if _, ok := SVGByStem("1f604"); ok {
		t.Error("SVGByStem answered")
	}
	if i, _ := Find("😄", 0); i != -1 {
		t.Error("Find answered")
	}
}

func TestBadStemName(t *testing.T) {
	// An archive whose key is not hexadecimal.
	reset(t, mustArchive(t, "zz"))
	if Err() == nil {
		t.Fatal("bad stem accepted")
	}
	reset(t, mustArchive(t, "d800")) // a surrogate is not a rune
	if Err() == nil {
		t.Fatal("surrogate accepted")
	}
}

func TestDuplicateNormalized(t *testing.T) {
	// "2764" and "2764-fe0f" are one sequence: the first stays.
	reset(t, mustArchive(t, "2764", "2764-fe0f"))
	if err := Err(); err != nil {
		t.Fatal(err)
	}
	if got := Lookup("❤"); got != "2764" {
		t.Errorf("Lookup = %q", got)
	}
}

func TestConcurrentUse(t *testing.T) {
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, s := range []string{"😄", "👩‍💻", "🇫🇷", "1️⃣", "❤️"} {
				if svg, ok := SVG(s); !ok || svg == "" {
					t.Error("missing", s)
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkCutText(b *testing.B) {
	s := strings.Repeat("Bonjour à tous, voici une diapositive élégante. ", 20)
	b.SetBytes(int64(len(s)))
	for range b.N {
		Find(s, 0)
	}
}

func BenchmarkSVG(b *testing.B) {
	for range b.N {
		SVG("😄")
	}
}
