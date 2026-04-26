package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/ai"
)

func init() {
	cmd := &cobra.Command{
		Use:   "ask [prompt...]",
		Short: "One-shot question, streamed to stdout",
		Long: `Ask sends a single prompt to Claude and streams the reply to stdout.

If no prompt is given on the command line, ask reads from stdin. Attach
images with --image / -i; useful for "what's in this meal?" lookups.`,
		Example: `  bite ask "kcal in 200g salmon?"
  echo "what should I eat for breakfast?" | bite ask
  bite ask -i meal.jpg "estimate calories and macros"
  bite ask -i https://example.com/food.png "what is this?"`,
		RunE: runAsk,
	}
	cmd.Flags().StringP("system", "s", "", "override the system prompt for this turn")
	cmd.Flags().StringSliceP("image", "i", nil, "attach an image (local path or URL); repeatable")
	rootCmd.AddCommand(cmd)
}

func runAsk(cmd *cobra.Command, args []string) error {
	prompt := strings.Join(args, " ")
	if prompt == "" {
		stdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		prompt = strings.TrimSpace(string(stdin))
	}

	images, _ := cmd.Flags().GetStringSlice("image")
	if prompt == "" && len(images) == 0 {
		return errors.New("nothing to send — pass a prompt, pipe stdin, or use --image")
	}

	ctx := cmd.Context()
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if sys, _ := cmd.Flags().GetString("system"); sys != "" {
		cfg.SystemPrompt = sys
	}

	client, err := openAIClient(ctx, cfg)
	if err != nil {
		return err
	}

	return streamAsk(ctx, cmd.OutOrStdout(), client, prompt, images)
}

// streamAsk builds the message and streams the reply to out.
func streamAsk(ctx context.Context, out io.Writer, client ai.Streamer, prompt string, images []string) error {
	msg := ai.Message{Role: ai.RoleUser, Content: prompt}
	for _, img := range images {
		msg.Attachments = append(msg.Attachments, imageAttachment(img))
	}

	ch, err := client.Stream(ctx, []ai.Message{msg})
	if err != nil {
		return err
	}

	for ev := range ch {
		switch {
		case ev.Err != nil:
			return ev.Err
		case ev.Done:
			fmt.Fprintln(out)
			return nil
		default:
			fmt.Fprint(out, ev.Delta)
		}
	}
	return nil
}

// imageAttachment treats values starting with http:// or https:// as URLs and
// everything else as a local filesystem path.
func imageAttachment(v string) ai.Attachment {
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return ai.Attachment{URL: v}
	}
	return ai.Attachment{Path: v}
}
