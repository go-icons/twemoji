# twemoji

[Twemoji](https://github.com/jdecked/twemoji) emoji graphics as embedded SVG, and the
`:shortcode:` table of [markdown-it-emoji](https://github.com/markdown-it/markdown-it-emoji),
for pure-Go programs (`CGO_ENABLED=0`, no network).

```go
import "github.com/go-icons/twemoji"

svg, ok := twemoji.SVG("👩‍💻")          // the SVG document, however FE0F is placed
seq, _ := twemoji.Shortcode("smile")  // "😄"
n := twemoji.Cut("😄 bonjour")        // 4: the emoji at the start of the text, in bytes
start, end := twemoji.Find(text, 0)   // the next emoji in text, or -1
stem := twemoji.Stem("🇫🇷")           // "1f1eb-1f1f7", the file name Twemoji gives it
```

It is a **data package**: it draws nothing. A renderer (go-gfx's SVG rasteriser, a
browser, `<img src="data:image/svg+xml;base64,…">`) turns the SVG into pixels.

| | |
|---|---|
| Graphics | 4,009 SVG, Twemoji **17.0.3** (Emoji 17.0) |
| Shortcodes | 1,903, markdown-it-emoji **3.0.0** |
| Embedded size | 2.1 MB for 10.1 MB of SVG (deflate, in 48 KB blocks of neighbouring emoji) |
| Start-up cost | none: the archive is parsed, and the lookup tables built, on first use |
| Lookup | `SVG` ≈ 38 ns (one 48 KB block inflated on a miss, 8 recently used kept) |
| Text scan | `Find` ≈ 300 MB/s on prose, no allocation |

## Attribution

The graphics are **CC-BY 4.0**: whoever shows them must credit them. Use
`twemoji.Attribution`:

> Twemoji, Copyright 2020 Twitter, Inc and other contributors (CC-BY 4.0), continued by the jdecked/twemoji project

The code of this module is BSD-3-Clause (`LICENSE`). The third-party files keep their licences:
`TWEMOJI-LICENSE-GRAPHICS` (CC-BY 4.0, the graphics), `TWEMOJI-LICENSE` (MIT, Twemoji's code),
`MARKDOWN-IT-EMOJI-LICENSE` (MIT, the shortcode table, itself derived from gemoji).

## Names

A graphic's **stem** is its file name in Twemoji: the code points in lower-case hexadecimal
joined with `-`, with U+FE0F dropped unless the sequence has a ZWJ: `😄` is `1f604`, `🇫🇷` is
`1f1eb-1f1f7`, `👩‍💻` is `1f469-200d-1f4bb`, `1️⃣` is `31-20e3`. `Stem` computes it exactly as
twemoji.js does; `Lookup` finds the stem the set actually holds, ignoring U+FE0F.

## Reading text the way Marp Core does

`Cut` and `Find` find emoji in text with the rules of Marp Core (emoji-regex, then Twemoji's
parser). They were compared with Marp Core 4.4.0 on all 6,380 spellings of the set (each
sequence as Twemoji names it, and with U+FE0F after its first character):

* the longest sequence the set has a graphic for;
* a character whose default presentation is text (`©` `™` `↔` `☺` `✔` `▶`, 207 of them:
  `TextDefault`) is an emoji **only with U+FE0F** after it: `✔` stays a glyph, `✔️` becomes
  an image. The exception is `❤`, which emoji-regex takes bare;
* a match followed by U+FE0E (text selector) is left alone;
* a lone regional indicator is not a flag; a bare digit, `#` or `*` is not a keycap.

Where it differs from Marp Core, it is to convert more: `☝️ ✌️ ✍️ ⛹️ 🏋️ 🏌️ 🕴️ 🕵️ 🖐️ ⛷️` (Marp
Core leaves them as text), and the few sequences Twemoji draws that Unicode does not list
(`👨‍🎄`). Marp Core also keeps a U+FE0F after about thirty BMP emoji (`⚽ ☕ ♈`) inside the match;
here it stays in the text, where it is invisible.

## Regenerating

```
git clone --depth 1 --branch v17.0.3 https://github.com/jdecked/twemoji
git clone --depth 1 --branch 3.0.0 https://github.com/markdown-it/markdown-it-emoji
# emoji-sequences.txt: unicode-org/unicodetools, unicodetools/data/emoji/17.0/
go run ./cmd/gen --twemoji ./twemoji --markdown-it-emoji ./markdown-it-emoji \
    --emoji-sequences emoji-sequences.txt
```

The archive format (`internal/pack`) sorts the documents by key and deflates blocks of about
48 KB: skin-tone and gender variants of one emoji are neighbours and share most of their markup,
so a block compresses far better than the files one by one. The reader checks every length and
offset before using it, bounds a block at 4 MB, and refuses an archive that disagrees with itself.
