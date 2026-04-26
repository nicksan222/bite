package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/config"
	"github.com/nicksan222/bite/internal/db"
	"github.com/nicksan222/bite/internal/media"
)

func init() {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks on bite's environment",
		Long: `doctor verifies that bite is ready to run.

Hard checks (cause non-zero exit if they fail):
  • config can be loaded
  • SQLite is reachable and migrations apply
  • ANTHROPIC_API_KEY is set
  • (with --ping) the model responds to a tiny test call

Soft checks (warnings only — features remain optional):
  • OPENAI_API_KEY  — needed for audio (Whisper)
  • ffmpeg in PATH  — needed for video keyframe extraction`,
		RunE: runDoctor,
	}
	cmd.Flags().BoolP("ping", "p", false, "send a tiny test request to the model")
	rootCmd.AddCommand(cmd)
}

type check struct {
	name string
	run  func(ctx context.Context) (detail string, err error)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	ping, _ := cmd.Flags().GetBool("ping")

	var cfg config.Config

	hard := []check{
		{name: "config: load", run: func(context.Context) (string, error) {
			var err error
			cfg, err = config.Load()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("DSN=%s model=%s", cfg.DSN, cfg.Model), nil
		}},
		{name: "config: api key", run: func(context.Context) (string, error) {
			if err := cfg.RequireAPIKey(); err != nil {
				return "", err
			}
			return fmt.Sprintf("ANTHROPIC_API_KEY set (len=%d)", len(cfg.APIKey)), nil
		}},
		{name: "db: open + migrate", run: func(ctx context.Context) (string, error) {
			store, err := db.Open(ctx, cfg.DSN)
			if err != nil {
				return "", err
			}
			_ = store.Close()
			return filepath.Base(cfg.DSN), nil
		}},
	}

	if ping {
		hard = append(hard, check{name: "ai: ping model", run: func(ctx context.Context) (string, error) {
			client, err := ai.NewClient(ctx, ai.ClientConfig{
				APIKey: cfg.APIKey, Model: cfg.Model, MaxTokens: 16,
			})
			if err != nil {
				return "", err
			}
			return pingModel(ctx, client)
		}})
	}

	soft := []check{
		{name: "media: openai key", run: func(context.Context) (string, error) {
			if cfg.OpenAIAPIKey == "" {
				return "", errors.New("not set — audio (Whisper) disabled")
			}
			return "set — audio transcription available", nil
		}},
		{name: "media: ffmpeg", run: func(context.Context) (string, error) {
			if err := media.CheckFFmpeg(); err != nil {
				return "", err
			}
			return "found — video keyframe extraction available", nil
		}},
	}

	failed := runChecks(ctx, out, hard, "✗")
	fmt.Fprintln(out)
	runChecks(ctx, out, soft, "!")

	if failed > 0 {
		fmt.Fprintf(out, "\n%d required check(s) failed.\n", failed)
		return fmt.Errorf("doctor: %d failed", failed)
	}
	fmt.Fprintln(out, "\nAll required checks passed.")
	return nil
}

func pingModel(ctx context.Context, streamer ai.Streamer) (string, error) {
	ch, err := streamer.Stream(ctx, []ai.Message{
		{Role: ai.RoleUser, Content: "say 'pong' and nothing else"},
	})
	if err != nil {
		return "", err
	}
	for ev := range ch {
		if ev.Err != nil {
			return "", ev.Err
		}
		if ev.Done {
			break
		}
	}
	return "model responded", nil
}

func runChecks(ctx context.Context, out io.Writer, checks []check, failGlyph string) int {
	failed := 0
	for _, c := range checks {
		detail, err := c.run(ctx)
		if err != nil {
			fmt.Fprintf(out, "  %s %-22s  %v\n", failGlyph, c.name, err)
			failed++
			continue
		}
		fmt.Fprintf(out, "  ✓ %-22s  %s\n", c.name, detail)
	}
	return failed
}
