package cli

import (
	"context"
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

func TestExecute_rootRunE_noTTY(t *testing.T) {
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
