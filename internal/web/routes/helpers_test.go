package routes

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// TestMergeArgs covers the precedence rule (form wins) and the empty-
// string drop. HTML forms send "" for unfilled fields; tools must see
// them as absent so set_goals' Has-vs-zero distinction survives.
func TestMergeArgs(t *testing.T) {
	got := mergeArgs(
		map[string]string{"a": "from-query", "b": "", "c": "kept"},
		map[string]string{"a": "from-form", "d": "", "e": "added"},
	)
	require.Equal(t, "from-form", got["a"], "form must win over query")
	require.NotContains(t, got, "b", "empty query value must be dropped")
	require.NotContains(t, got, "d", "empty form value must be dropped")
	require.Equal(t, "kept", got["c"])
	require.Equal(t, "added", got["e"])
}

// TestNotFoundError_message pins that NotFoundError carries the
// requested name in its message — handlers branch on errors.As and rely
// on the message for the user-facing 404 body.
func TestNotFoundError_message(t *testing.T) {
	err := NotFoundError{Name: "missing_tool"}
	require.Contains(t, err.Error(), "missing_tool")
}

// TestToolErrorStatus_mapping locks in the contract every tool surface
// (JSON and HTMX) shares: NotFoundError → 404, anything else → 400.
// Wrapped NotFoundError must still resolve to 404 — proves the helper
// uses errors.As (unwrap-aware), not a direct type assertion.
func TestToolErrorStatus_mapping(t *testing.T) {
	require.Equal(t, http.StatusNotFound, toolErrorStatus(NotFoundError{Name: "foo"}))
	require.Equal(t, http.StatusBadRequest, toolErrorStatus(errors.New("kcal must be positive")))
	require.Equal(t, http.StatusNotFound,
		toolErrorStatus(wrappedTestErr{NotFoundError{Name: "x"}}))
}

// wrappedTestErr is a one-line Unwrap wrapper used only by the
// toolErrorStatus test.
type wrappedTestErr struct{ inner error }

func (w wrappedTestErr) Error() string { return "wrapped: " + w.inner.Error() }
func (w wrappedTestErr) Unwrap() error { return w.inner }

// TestRender_unknownTemplateReturnsFiberError documents the
// programmer-error path: render() called with a name that doesn't exist
// in pageTemplates returns a 500 fiber.Error rather than panicking.
// This branch fires only on a typo in a page handler — easy to make,
// easy to miss without the explicit error.
func TestRender_unknownTemplateReturnsFiberError(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return render(c, "no_such_page.html", nil) })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Direct call form: covers the error message itself, which the
	// app.Test path discards.
	got := render(nil, "no_such_page.html", nil)
	var fe *fiber.Error
	require.True(t, errors.As(got, &fe))
	require.Contains(t, fe.Message, "no_such_page.html")
}
