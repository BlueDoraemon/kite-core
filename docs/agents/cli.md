# CLI

`kite` is the command-line agent.

## Commands

| Command | Purpose |
| --- | --- |
| `kite run [flags] <prompt>` | Run a prompt in the current directory |
| `kite tui [flags] [session-id]` | Open or resume the interactive terminal workspace |
| `kite lint [flags] [path ...]` | Run layered repository style checks |
| `kite resume <session-id> [prompt]` | Resume a session |
| `kite rpc` | Serve the NDJSON RPC protocol on stdin/stdout |
| `kite status [session-id]` | Show session status |
| `kite inspect <tool-id>` | Show a tool's schema |
| `kite artifact [--offset N --limit N] <artifact-id>` | Retrieve an artifact |
| `kite context [--full] [session-id]` | Show the session context |

## Flags

| Flag | Applies to | Purpose |
| --- | --- | --- |
| `-base-url <url>` | run, tui, resume, rpc, lint with `-llm` | OpenAI-compatible API base URL |
| `-model <id>` | run, tui, resume, rpc, lint with `-llm` | Model identifier |
| `-from-crush` | run, tui, resume, rpc, lint with `-llm` | Import model, credential, and endpoint from Crush |
| `-no-print` | run | Do not mirror output to stdout |
| `-theme <name>` | tui | Select `night-flight`, `paper-trail`, or `high-contrast` |
| `-plain` | tui | Disable ANSI colour and screen clearing |
| `-json` | lint | Emit the `kite.lint/v1` JSON contract |
| `-max-line <n>` | lint | Set maximum line length; `0` disables it |
| `-vale` | lint | Include an installed Vale CLI's JSON alerts |
| `-vale-bin <path>` | lint | Select the Vale executable |
| `-llm` | lint | Include bounded provider-backed style review |
| `-llm-strict` | lint | Let LLM warnings affect the exit code |
| `-offset N` | artifact | Byte offset to read from |
| `-limit N` | artifact | Maximum bytes to read |
| `-full` | context | Show full context including repository instructions |

Flags must precede positional arguments. For example, use
`kite artifact --offset 32768 art_...` and `kite context --full sess_...`.

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `KITE_API_KEY` | unset | API key sent as a Bearer token |
| `KITE_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible API base URL |
| `KITE_MODEL` | `gpt-4o-mini` | Model identifier |
| `KITE_DATA_DIR` | XDG/LOCALAPPDATA | Where sessions and artifacts are stored |
| `KITE_THEME` | `night-flight` | Default TUI theme |
| `NO_COLOR` | unset | Disable TUI colour when set |

Configuration precedence is flags > environment > config file > defaults. The
config file lives at `$XDG_CONFIG_HOME/kite/config.json` (Unix) or
`%APPDATA%\kite\config.json` (Windows); see [Providers](providers.md).

## Exit codes

- `0` — completed
- `1` — runtime, verification, or lint failure
- `2` — usage or configuration error

## Human output

`kite run` mirrors the model's text to stdout as it streams, then prints a
structured result block. `kite rpc` keeps stdout protocol-only; all
diagnostics go to stderr.

`kite tui` renders the same durable events as an interactive hunk ledger. It
does not create a separate execution path. See [Terminal UI](tui.md) for its
commands, themes, fallbacks, and safety behaviour.

`kite lint` is offline and deterministic unless `-vale` or `-llm` is
selected. Vale alerts retain their rule names; model findings are visibly
advisory unless `-llm-strict` is selected. See [Layered lint](lint.md).

## `--from-crush`

Reads the persisted Crush-selected large model, credential, and cached
endpoint without executing crushrc. Supports OpenAI, OpenAI-compatible, and
Hyper providers. Rejects unsupported providers, missing endpoints, and
expired or near-expiry OAuth credentials with actionable secret-free errors.
