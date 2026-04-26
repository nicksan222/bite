# bite

A terminal-first AI nutritionist. Chat in your terminal — log meals, track calories and macros, set goals.

```
$ bite chat
> had 200g of pasta with pesto for lunch
  Logged. ~480 kcal · 62g carbs · 14g fat · 12g protein.
  You're at 1,240 / 2,000 kcal today.
```

## Requirements

- Go 1.25+
- An API key for one of the supported providers (Anthropic, OpenAI, Gemini),
  **or** a local [Ollama](https://ollama.com) install — bite can offer to set
  one up for you on first run.

## Quickstart

```bash
git clone https://github.com/nicksan222/bite
cd bite

cp .env.example .env
# add ANTHROPIC_API_KEY (or OPENAI_API_KEY / GEMINI_API_KEY) to .env
# — or skip this step and bite will offer to install Ollama locally on first run

make setup
make run
```

## Configuration

All config lives in `.env` (or environment variables):

| Variable | Default | Description |
|---|---|---|
| `BITE_PROVIDER` | auto-detect | `anthropic` · `openai` · `gemini` · `ollama`. Empty picks the first provider whose key is set. |
| `ANTHROPIC_API_KEY` | — | Anthropic key (default provider). |
| `OPENAI_API_KEY` | — | OpenAI key. Also enables Whisper audio transcription. |
| `GEMINI_API_KEY` | — | Google Gemini key. |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Local Ollama daemon URL. |
| `BITE_DB` | `$XDG_DATA_HOME/bite/bite.db` | SQLite database path. |
| `BITE_MODEL` | provider's default | Override the model ID for the active provider. |
| `BITE_MAX_TOKENS` | `4096` | Max tokens per turn. |
| `BITE_SYSTEM_PROMPT` | built-in | Override the nutritionist prompt. |

## Commands

```bash
make run     # open chat TUI
make test    # run tests
make lint    # golangci-lint
make build   # build ./bin/bite
```

Every domain action is also a top-level subcommand and a slash command inside chat — both auto-generated from the same `internal/tools` registry:

```bash
bite log_meal "200g pasta with pesto"          # CLI
bite meals_today                               # today's intake summary
bite ask "kcal in 200g salmon?"                # one-shot question

# inside chat
> /log_meal pasta kcal=480
> /help                                        # list every slash command
```

Run `bite doctor` if something feels off — the check list is auto-derived from registered checks and grows whenever `tools.RegisterCheck` is added.

## Stack

Go · [eino](https://github.com/cloudwego/eino) (Anthropic · OpenAI · Gemini · Ollama) · [bubbletea](https://github.com/charmbracelet/bubbletea) · SQLite
