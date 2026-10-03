package localfile

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/n-r-w/classifier-mcp/internal/domain"
	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// Read preserves text bytes and caller boundaries; acquisition failures affect one source.
func (s *Service) Read(
	ctx context.Context,
	source domain.FileSource,
) (classify.AcquiredSource, error) {
	if err := ctx.Err(); err != nil {
		return classify.AcquiredSource{}, err
	}
	if strings.TrimSpace(source.Path) == "" {
		return classify.AcquiredSource{}, errors.New("file path must not be blank")
	}
	if lines, present := source.Lines.Get(); present && (lines.Start.Sign() < 1 || lines.End.Cmp(lines.Start) < 0) {
		return classify.AcquiredSource{}, fmt.Errorf(
			"requested lines %d through %d in %s; require 1 <= start <= end",
			lines.Start,
			lines.End,
			source.Path,
		)
	}
	path, resolveErr := resolvePath(source.Path)
	if resolveErr != nil {
		return classify.AcquiredSource{}, resolveErr
	}
	// The MCP schema requires an explicit file source; Read resolves its caller-selected local path.
	data, err := os.ReadFile(path) //nolint:gosec // The file-source contract permits any caller-selected local path.
	if canceled := ctx.Err(); canceled != nil {
		return classify.AcquiredSource{}, canceled
	}
	if err != nil {
		return classify.AcquiredSource{}, err
	}
	if !utf8.Valid(data) {
		return classify.AcquiredSource{}, fmt.Errorf("read %s as text: invalid UTF-8", path)
	}
	content := string(data)
	if lines, present := source.Lines.Get(); present {
		fragment, rangeErr := selectLines(content, lines, path)
		if rangeErr != nil {
			return classify.AcquiredSource{}, rangeErr
		}
		content = fragment
	}
	if canceled := ctx.Err(); canceled != nil {
		return classify.AcquiredSource{}, canceled
	}
	return classify.AcquiredSource{
		Content: content,
		Source:  domain.FileSource{Path: path, Lines: source.Lines},
	}, nil
}

// resolvePath makes native relative references absolute while retaining filesystem traversal segments.
func resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	volume := filepath.VolumeName(path)
	tail := path[len(volume):]
	var base string
	var err error
	switch {
	case tail != "" && os.IsPathSeparator(tail[0]):
		// A Windows root-relative reference inherits the native current drive, not its directory.
		base, err = filepath.Abs(volume + tail[:1])
		tail = tail[1:]
	case volume != "":
		// A drive-relative reference inherits that drive's native current directory.
		base, err = filepath.Abs(volume + ".")
	default:
		base, err = os.Getwd()
	}
	if err != nil {
		return "", fmt.Errorf("resolve file path %q: %w", path, err)
	}
	// Cleaning the tail before opening it would change the meaning of symlink/.. traversal.
	if !os.IsPathSeparator(base[len(base)-1]) {
		base += string(os.PathSeparator)
	}
	return base + tail, nil
}

// selectLines retains line terminators and counts an empty file as zero lines.
func selectLines(content string, lines domain.LineRange, path string) (string, error) {
	parts := strings.SplitAfter(content, "\n")
	count := len(parts)
	if parts[count-1] == "" {
		count--
	}
	if lines.End.Cmp(big.NewInt(int64(count))) > 0 {
		return "", fmt.Errorf(
			"requested lines %d through %d in %s; the file has %d lines",
			lines.Start,
			lines.End,
			path,
			count,
		)
	}
	// Existing boundaries fit slice indices because each is bounded by the file's line count.
	start := int(lines.Start.Int64())
	end := int(lines.End.Int64())
	return strings.Join(parts[start-1:end], ""), nil
}
