package localfile

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/suite"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// sourceSuite checks text acquisition at the filesystem boundary.
type sourceSuite struct{ suite.Suite }

// TestSources covers line semantics, decoding failures, and boundary cancellation.
func TestSources(t *testing.T) { t.Parallel(); suite.Run(t, new(sourceSuite)) }

// TestExactLines checks empty files, final terminators, blank lines, and unbounded line length.
func (s *sourceSuite) TestExactLines() {
	for _, test := range []struct {
		// name identifies the observable file case.
		name string
		// text contains the exact bytes stored in the fixture.
		text string
		// lines contains inclusive caller boundaries.
		lines domain.LineRange
		// want is the exact selected text.
		want string
		// fails marks a requested boundary that does not exist.
		fails bool
	}{
		{name: "empty whole", text: "", lines: domain.LineRange{}, want: "", fails: false},
		{
			name: "empty range", text: "", lines: domain.LineRange{Start: big.NewInt(1), End: big.NewInt(1)},
			want: "", fails: true,
		},
		{
			name: "final newline", text: "one\ntwo\n", lines: domain.LineRange{Start: big.NewInt(2), End: big.NewInt(2)},
			want: "two\n", fails: false,
		},
		{
			name: "no phantom line", text: "one\ntwo\n", lines: domain.LineRange{Start: big.NewInt(3), End: big.NewInt(3)},
			want: "", fails: true,
		},
		{
			name: "blank line", text: "\n\nlast", lines: domain.LineRange{Start: big.NewInt(2), End: big.NewInt(2)},
			want: "\n", fails: false,
		},
		{
			name: "long line", text: strings.Repeat("x", 100000) + "\nlast",
			lines: domain.LineRange{Start: big.NewInt(1), End: big.NewInt(1)},
			want:  strings.Repeat("x", 100000) + "\n", fails: false,
		},
	} {
		s.Run(test.name, func() {
			path := filepath.Join(s.T().TempDir(), "text")
			s.Require().NoError(os.WriteFile(path, []byte(test.text), 0o600))
			lines := mo.Some(test.lines)
			if test.name == "empty whole" {
				lines = mo.None[domain.LineRange]()
			}
			acquired, diagnostic := New().Read(s.T().Context(), domain.FileSource{Path: path, Lines: lines})
			if test.fails {
				s.Require().True(diagnostic.IsSome())
				s.Equal("invalid_source", diagnostic.MustGet().Code)
			} else {
				s.Require().True(diagnostic.IsNone())
				s.Equal(test.want, acquired.Content)
				s.Equal(path, acquired.Source.Path)
				s.Equal(lines, acquired.Source.Lines)
			}
		})
	}
}

// TestReadFailuresAndCancellation checks concrete OS and decoding causes without model attempts.
func (s *sourceSuite) TestReadFailuresAndCancellation() {
	path := filepath.Join(s.T().TempDir(), "invalid")
	s.Require().NoError(os.WriteFile(path, []byte{0xff}, 0o600))
	_, diagnostic := New().Read(s.T().Context(), domain.FileSource{Path: path, Lines: mo.None[domain.LineRange]()})
	s.Require().True(diagnostic.IsSome())
	s.Equal("source_read_failed", diagnostic.MustGet().Code)
	s.Contains(diagnostic.MustGet().Message, "invalid UTF-8")
	s.Contains(diagnostic.MustGet().Message, path)
	_, directoryDiagnostic := New().Read(
		s.T().Context(), domain.FileSource{Path: s.T().TempDir(), Lines: mo.None[domain.LineRange]()},
	)
	s.Require().True(directoryDiagnostic.IsSome())
	s.Equal("source_read_failed", directoryDiagnostic.MustGet().Code)
	ctx, cancel := context.WithCancel(s.T().Context())
	cancel()
	_, canceled := New().Read(ctx, domain.FileSource{Path: path, Lines: mo.None[domain.LineRange]()})
	s.Require().True(canceled.IsSome())
	s.Equal("canceled", canceled.MustGet().Code)
	s.Equal("read_source", canceled.MustGet().Operation)
	s.Equal(context.Canceled.Error(), canceled.MustGet().Message)
	s.True(canceled.MustGet().Attempts.IsNone())
}

// TestFreshReads checks that repeated references obtain the contents of each call.
func (s *sourceSuite) TestFreshReads() {
	path := filepath.Join(s.T().TempDir(), "text")
	reader := New()
	for _, text := range []string{"before", "after"} {
		s.Require().NoError(os.WriteFile(path, []byte(text), 0o600))
		acquired, diagnostic := reader.Read(
			s.T().Context(),
			domain.FileSource{Path: path, Lines: mo.None[domain.LineRange]()},
		)
		s.Require().True(diagnostic.IsNone())
		s.Equal(text, acquired.Content)
	}
}

// TestAbsolutePathPreserved checks direct use of an absolute path with parent traversal.
func (s *sourceSuite) TestAbsolutePathPreserved() {
	directory := s.T().TempDir()
	s.Require().NoError(os.Mkdir(filepath.Join(directory, "child"), 0o700))
	s.Require().NoError(os.WriteFile(filepath.Join(directory, "text"), []byte("content"), 0o600))
	separator := string(os.PathSeparator)
	path := filepath.Join(directory, "child") + separator + ".." + separator + "text"
	acquired, diagnostic := New().Read(s.T().Context(), domain.FileSource{Path: path, Lines: mo.None[domain.LineRange]()})
	s.Require().True(diagnostic.IsNone())
	s.Equal("content", acquired.Content)
	s.Equal(path, acquired.Source.Path)
}
