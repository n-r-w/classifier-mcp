// Package localfile acquires local UTF-8 text and exact inclusive line fragments.
package localfile

import "github.com/n-r-w/classifier-mcp/internal/usecases/classify"

// Service reads each file reference using the process working directory.
type Service struct{}

// The reader implements the acquisition contract owned by its classification consumer.
var _ classify.ISourceReader = (*Service)(nil)

// New constructs the local file reader.
func New() *Service { return &Service{} }
