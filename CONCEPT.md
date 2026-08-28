# Concept: Kite out-of-the-box

## Vision

Kite works the moment it is installed, for everyone. Developers get a guided
setup and their own provider; non-developers get a zero-config free path; a
paid tier keeps the free path alive and funds ongoing support.

## The gap today

`kite run` needs a model and a credential before it can do anything:

- `KITE_API_KEY` env var, defaulting to `gpt-4o-mini` on
  `https://api.openai.com/v1` (cmd/kite/main.go:244-264).
- Or `--from-crush`, which only works for people who already use Crush.
- No config file, no setup command, no auth flow, no connection test.

The session log records that nobody has run `kite run` against a real model
yet (SESSION_LOG.md:12). A developer who already has an OpenAI key and knows
how to set env vars can get value; a non-developer is locked out.

## The three phases

1. **Developer setup polish** — the "standard" bar. Guided setup, config
   file, provider presets, connection test. Cheap, fits the principles, and
   the config file is the seam everything else hangs off.
2. **Zero-config free default** — a hosted relay so `kite run "..."` just
   works with no key. The non-developer on-ramp. Requires a decision about
   who pays and how abuse is controlled.
3. **Paid option** — funds the free tier and provides ongoing support.

The sequencing protects the project: phase 1 is self-sufficient and needs no
strategic decision, so the project is never hostage to the hosted service.

---

## Phase 1 scope (this document)

### Goal

A first-time developer can install Kite, run `kite setup`, and complete a
first `kite run` without reading the docs. A developer who already has a key
can be productive in under two minutes.

### What is in scope

- **`kite setup`** — an interactive wizard that:
  - Detects existing configuration (env vars, config file, `--from-crush`
    availability).
  - Lists provider presets (OpenAI, Anthropic, Groq, OpenRouter, Ollama,
    custom) with their base URLs and sensible model defaults.
  - Walks through getting a key for the chosen provider.
  - Tests the connection with a minimal request before writing anything.
  - Writes a config file with user-only permissions.
  - Supports a non-interactive mode for scripting (`kite setup -provider
    openai -key sk-...`).
- **Config file** — `~/.config/kite/config.json` (Unix) and the platform
  equivalent on Windows, user-only permissions, with precedence:
  `flags > env > config file > defaults`.
- **Provider presets** — a small table of known providers. Each preset knows
  its base URL and a sensible default model.
- **Connection test** — a lightweight provider round-trip used by the wizard
  so a bad key or endpoint is caught at setup time, not on the first real
  run.
- **First-run guidance** — `kite run` with no configuration says "run
  `kite setup`" instead of failing cryptically.
- **Documentation** — a setup page, updated quickstart, CLI, providers,
  security, and troubleshooting docs.

### What is out of scope (phase 2)

- Any hosted relay, free default endpoint, or `kite login`.
- Any billing, accounts, or usage metering.
- Any change to the runtime, event contracts, or provider wire format.

### Design constraints

- Standard library only. No new third-party dependencies.
- The config file must never store secrets insecurely; the API key may be
  stored in the config file with user-only permissions, or referenced from
  an environment variable.
- Secrets never appear in output or persisted errors.
- The config file is additive and versioned (`kite.config/v1`), so a future
  phase can extend it without breaking existing installs.
- Cross-platform: the config path and permissions must work on Linux, macOS,
  and Windows.
- Existing env-var and `--from-crush` behaviour must keep working unchanged.

### Files touched

| Area | Change |
| --- | --- |
| `internal/config` (new) | Config file load/save, precedence, presets, connection test |
| `cmd/kite/main.go` | `kite setup` command, `providerConfig` reads config file, first-run guidance |
| `docs/agents/setup.md` (new) | Setup guide |
| `docs/agents/cli.md` | Document `kite setup` |
| `docs/agents/providers.md` | Document config file precedence |
| `docs/agents/quickstart.md` | Lead with `kite setup` |
| `docs/agents/security.md` | Config file credential handling |
| `docs/agents/troubleshooting.md` | Setup and config-file failure modes |
| `README.md` | Quick start leads with `kite setup` |

### Validation

- `go build ./...`, `go vet ./...`, `go test ./...` all green.
- Cross-compile every release target.
- New behavioural tests for: config precedence, preset resolution, connection
  test against a scripted endpoint, and `kite setup` non-interactive mode.

---

## Phase 2 sketch (not built here)

A hosted Kite-compatible relay in front of a model, used as the default when
no key is present. `kite run "explain this repo"` just works. Free tier with
rate limits, and a clear bring-your-own-key upgrade path. `kite login` for
the hosted service becomes the seam into phase 3.

Open questions that phase 2 must answer:

- Who pays for the relay? (Phase 3 funds it.)
- Abuse and cost control: per-user limits, rate limiting that degrades
  gracefully, not silent stalls.
- Privacy: prompts now go through a third party; needs explicit disclosure.
- Reliability: it is now a public service with uptime expectations.

## Phase 3 sketch (not built here)

The business model options, roughly in order of simplicity:

| Model | How it works | Fit |
| --- | --- | --- |
| Freemium hosted | Free tier (rate-limited) + paid tier (higher limits, priority) | Simplest, matches the free-for-everyone vision |
| BYOK + hosted | Free for bring-your-own-key users, paid for hosted usage | Keeps the community ethos |
| Usage-based | `kite login`, pay per token | Most flexible, most complex |
| Support/enterprise | Support contracts, SLAs, self-hosted licensing | For teams, later |

## The strategic tension

Today PRODUCT.md says "no commercial claims" and AGENTS.md lists no
CI-delivery product features. A paid tier turns Kite from community-built
infrastructure into open-core with a hosted service. That is a deliberate
identity decision, not an accident. The sequencing protects the project:
phase 1 needs no such decision; phases 2 and 3 do, and by then there is real
usage data to justify it.
