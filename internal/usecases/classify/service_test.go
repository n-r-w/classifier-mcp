package classify

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/n-r-w/classifier-mcp/internal/domain"
	"github.com/n-r-w/classifier-mcp/internal/server"
)

// executionSuite checks concurrent object isolation and cancellation with controlled model completions.
type executionSuite struct {
	// Suite supplies assertions and lifecycle state for ordered object execution cases.
	suite.Suite
}

// TestExecution runs the ordered execution scenarios.
func TestExecution(t *testing.T) { t.Parallel(); suite.Run(t, new(executionSuite)) }

// TestOrderedCompletion keeps a completed success when cancellation interrupts a slower earlier object.
func (s *executionSuite) TestOrderedCompletion() {
	synctest.Test(s.T(), func(t *testing.T) {
		ctrl := gomock.NewController(t)
		model := NewMockIModel(ctrl)
		reader := NewMockISourceReader(ctrl)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		completed := make(chan struct{})
		model.EXPECT().Evaluate(gomock.Any(), gomock.Any()).DoAndReturn(
			func(callCtx context.Context, request Request) (Response, mo.Option[domain.Diagnostic]) {
				if request.Content == "slow" {
					<-callCtx.Done()
					return Response{}, mo.Some(domain.Diagnostic{
						Code:              "canceled",
						Operation:         "classify",
						Message:           callCtx.Err().Error(),
						HTTPStatus:        mo.None[int](),
						UpstreamBody:      mo.None[string](),
						UpstreamRequestID: mo.None[string](),
						Attempts:          mo.None[int](),
						RetryAfterSeconds: mo.None[float64](),
					})
				}
				close(completed)
				return Response{
					Model:   "actual",
					Answers: nil,
					Usage:   mo.None[domain.Usage](),
				}, mo.None[domain.Diagnostic]()
			}).Times(2)
		command := server.Command{Objects: []server.Object{
			{ID: "slow", Source: mo.Left[domain.TextSource, domain.FileSource](domain.TextSource{Text: "slow"})},
			{ID: "fast", Source: mo.Left[domain.TextSource, domain.FileSource](domain.TextSource{Text: "fast"})},
		}, Task: "task", Questions: nil, Full: false}
		service := New(model, reader, 2)
		result := make(chan []server.Outcome, 1)
		go func() { result <- service.Classify(ctx, command) }()
		synctest.Wait()
		select {
		case <-completed:
		default:
			assert.Fail(t, "later object must complete while the earlier object is blocked")
		}
		cancel()
		outcomes := <-result
		require.Len(t, outcomes, 2)
		failure, ok := outcomes[0].Right()
		require.True(t, ok)
		require.Equal(t, "slow", failure.ID)
		success, ok := outcomes[1].Left()
		require.True(t, ok)
		require.Equal(t, "fast", success.ID)
	})
}
