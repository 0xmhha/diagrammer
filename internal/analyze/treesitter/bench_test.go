//go:build cgo

package treesitter_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/diagrammer/internal/analyze"
	"github.com/0xmhha/diagrammer/internal/analyze/treesitter"
)

// What the second build costs, in a form that can be taken again.
//
// docs/decisions.md carried a speed and a size for this runtime that came from
// the project that publishes it and had never been taken here, which is the one
// thing docs/thresholds.md exists to forbid. This is how the speed half is
// taken. The size half is `make size`.
//
// It reports bytes per second over the committed fixtures, repeated until there
// is enough of them to measure. Repeating one file is a fair measure of a
// parser's throughput and an unfair measure of almost anything else, so nothing
// else is claimed from it.

// benchCopies is how many times each fixture is repeated. The fixtures are a
// few hundred bytes each, which is startup cost and noise; this is enough to
// put the work where the measurement is looking.
const benchCopies = 200

func BenchmarkAnalyze(b *testing.B) {
	for _, a := range treesitter.Analyzers() {
		b.Run(string(a.Language()), func(b *testing.B) {
			dir, bytes := repeatedFixture(b, a.Extensions())
			if bytes == 0 {
				b.Skipf("no fixture for %s", a.Language())
			}
			b.SetBytes(bytes)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := a.Analyze(context.Background(), dir, analyze.Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// repeatedFixture builds a tree of copies of every fixture one analyzer claims,
// and says how many bytes it holds.
func repeatedFixture(b *testing.B, extensions []string) (string, int64) {
	claims := map[string]bool{}
	for _, e := range extensions {
		claims[e] = true
	}

	// Two roots. The polyglot fixture covers three of the four languages; its
	// only JavaScript is the two files written to be unreadable, so the
	// viewer, which is this project's own JavaScript, stands in for that one.
	roots := []string{fixture(), filepath.Join("..", "..", "render", "viewer")}

	var sources [][]byte
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() || !claims[filepath.Ext(path)] {
			return err
		}
		// A file the grammar cannot read is a fixture for the diagnostics and
		// noise for a measurement of how fast it reads what it can.
		if strings.Contains(entry.Name(), "broken") || strings.Contains(entry.Name(), "nul-") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sources = append(sources, content)
		return nil
	}
	for _, root := range roots {
		if err := filepath.WalkDir(root, walk); err != nil {
			b.Fatalf("read %s: %v", root, err)
		}
	}

	dir := b.TempDir()
	var total int64
	for i, content := range sources {
		for copy := range benchCopies {
			name := filepath.Join(dir, "f"+itoa(i)+"_"+itoa(copy)+claimedExt(extensions))
			if err := os.WriteFile(name, content, 0o600); err != nil {
				b.Fatalf("write: %v", err)
			}
			total += int64(len(content))
		}
	}
	return dir, total
}

// claimedExt is the first extension an analyzer claims, which is the one the
// copies are named with.
func claimedExt(extensions []string) string {
	if len(extensions) == 0 {
		return ""
	}
	return extensions[0]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
