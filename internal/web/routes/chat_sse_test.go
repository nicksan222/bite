package routes

import (
	"bufio"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

// TestPumpStreamEvents_assemblesWhenFinalEmpty pins the fallback: if
// the model reports Done with an empty Final, the pump returns the
// concatenation of every Delta — otherwise the session would record an
// empty assistant turn even though the user clearly received tokens.
func TestPumpStreamEvents_assemblesWhenFinalEmpty(t *testing.T) {
	ch := make(chan ai.StreamEvent, 3)
	ch <- ai.StreamEvent{Delta: "hello"}
	ch <- ai.StreamEvent{Delta: " there"}
	ch <- ai.StreamEvent{Done: true} // Final left empty
	close(ch)

	var buf strings.Builder
	w := bufio.NewWriter(&buf)
	final := pumpStreamEvents(w, ch)
	require.Equal(t, "hello there", final)
}

// TestPumpStreamEvents_channelCloseWithoutDone covers the path where
// the model channel closes naturally (no terminal Done event) — pump
// should still hand back the accumulated text so the session can store
// the partial reply.
func TestPumpStreamEvents_channelCloseWithoutDone(t *testing.T) {
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Delta: "partial"}
	close(ch)

	var buf strings.Builder
	w := bufio.NewWriter(&buf)
	final := pumpStreamEvents(w, ch)
	require.Equal(t, "partial", final)
}

// TestPumpStreamEvents_clientDisconnectStopsLoop proves that once a
// write to the SSE stream fails (client closed the connection), the
// pump exits immediately rather than continuing to drain the channel.
// Otherwise a slow-disconnecting client would force the model goroutine
// to keep producing tokens we'd silently throw away.
func TestPumpStreamEvents_clientDisconnectStopsLoop(t *testing.T) {
	ch := make(chan ai.StreamEvent, 5)
	for i := 0; i < 5; i++ {
		ch <- ai.StreamEvent{Delta: "tok"}
	}
	close(ch)

	w := bufio.NewWriter(&failingWriter{failAt: 1})
	final := pumpStreamEvents(w, ch)
	require.Empty(t, final, "client-disconnect path must not return a final string")
	require.NotEmpty(t, ch, "pump must stop draining the channel once write fails")
}

// TestPumpStreamEvents_terminalErrorYieldsErrorEvent validates the
// mid-stream failure path: an ev.Err drains as `event: error` followed
// by `event: done` (matching the contract writeSSEErrorAndDone uses
// for pre-stream failures), and the pump returns "" so the session
// does NOT get a partial assistant message appended.
func TestPumpStreamEvents_terminalErrorYieldsErrorEvent(t *testing.T) {
	ch := make(chan ai.StreamEvent, 2)
	ch <- ai.StreamEvent{Delta: "part"}
	ch <- ai.StreamEvent{Err: errors.New("boom")}
	close(ch)

	var buf strings.Builder
	w := bufio.NewWriter(&buf)
	final := pumpStreamEvents(w, ch)
	require.Empty(t, final)
	out := buf.String()
	require.Contains(t, out, "event: error")
	require.Contains(t, out, "data: boom")
	require.Contains(t, out, "event: done",
		"mid-stream errors must terminate with a done event so sse-close fires cleanly")
}

// failingWriter errors on the Nth write. Drives pumpStreamEvents into
// its Flush-error early return.
type failingWriter struct {
	written int
	failAt  int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.written++
	if f.written >= f.failAt {
		return 0, errors.New("disconnected")
	}
	return len(p), nil
}
