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
- An [Anthropic API key](https://console.anthropic.com/)

## Quickstart

```bash
git clone https://github.com/nicksan222/bite
cd bite

cp .env.example .env
# add your ANTHROPIC_API_KEY to .env

make setup
make run
```

## Configuration

All config lives in `.env` (or environment variables):

| Variable | Default | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | — | **Required.** Your Anthropic key. |
| `BITE_DB` | `$XDG_DATA_HOME/bite/bite.db` | SQLite database path. |
| `BITE_MODEL` | `claude-sonnet-4-6` | Claude model ID. |
| `BITE_MAX_TOKENS` | `4096` | Max tokens per turn. |
| `BITE_SYSTEM_PROMPT` | built-in | Override the nutritionist prompt. |

## Commands

```bash
make run     # open chat TUI
make test    # run tests
make lint    # golangci-lint
make build   # build ./bin/bite
```

Run `bite doctor` if something feels off.

## Stack

Go · [eino](https://github.com/cloudwego/eino) · Claude · [bubbletea](https://github.com/charmbracelet/bubbletea) · SQLite
