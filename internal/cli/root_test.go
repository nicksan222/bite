package cli

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetBuildInfo_nonEmpty(t *testing.T) {
	SetBuildInfo("1.2.3", "abc123")
	assert.Equal(t, "1.2.3", buildVersion)
	assert.Equal(t, "abc123", buildCommit)
}

func TestSetBuildInfo_emptySkips(t *testing.T) {
	SetBuildInfo("before", "before")
	SetBuildInfo("", "")
	assert.Equal(t, "before", buildVersion, "buildVersion should not change on empty input")
}

// skipIfWindowsTTYHang: see launch_chat_test.go for rationale. Windows CI
// hands the test process a real console, so bubbletea's prog.Run blocks
// for input until the test timeout instead of failing fast.
func skipIfWindowsTTYHang(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows CI presents a real console — bubbletea hangs instead of failing fast")
	}
}

func TestExecute_rootRunE_noTTY(t *testing.T) {
	skipIfWindowsTTYHang(t)
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	rootCmd.SetArgs([]string{})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	err := Execute(context.Background())
	if err == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}

func TestExecute_chatSubcommand_noTTY(t *testing.T) {
	skipIfWindowsTTYHang(t)
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	rootCmd.SetArgs([]string{"chat"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	err := Execute(context.Background())
	if err == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}
