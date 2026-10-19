package twemoji

import (
	"testing"

	"github.com/go-icons/twemoji/internal/pack"
)

// mustArchive builds an archive of empty SVGs named stems.
func mustArchive(t *testing.T, stems ...string) string {
	t.Helper()
	var es []pack.Entry
	for _, s := range stems {
		es = append(es, pack.Entry{Key: s, Data: []byte("<svg/>")})
	}
	b, err := pack.Encode(es)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
