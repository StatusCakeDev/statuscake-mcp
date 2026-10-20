package tools_test

import (
	"testing"

	"github.com/StatusCakeDev/statuscake-mcp/internal/tools"
)

func TestPingAnswersPong(t *testing.T) {
	t.Parallel()

	h := tools.NewPingHandler(discardLogger())

	_, got, err := h.Handle(t.Context(), nil, tools.PingInput{})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if got.Message != "pong" {
		t.Errorf("Message = %q, want pong", got.Message)
	}
}
