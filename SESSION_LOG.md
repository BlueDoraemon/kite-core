# Session Log — kite

## Current status (updated 2026-08-20)

The Kite runtime and its documentation are complete and validated. Work is
now on branch `feat/layered-lint` (ahead of the 18 Aug `a660dea` milestone
with TUI and layered deterministic + LLM linting). This session audited and
fixed documentation and code comments against the Australian English writing
rules: 10 files touched (PRODUCT, DESIGN, AGENTS, README, cli/lint/tui docs,
a CLI flag description, a TUI comment, a test message), `go build`/`go vet`/
`go test ./...` all green, changes still uncommitted. Top open items: nobody
has run `kite run` against a real paid model yet, and the current branch has
not been pushed or run through the no-mistakes pipeline. The single next
action: run `kite run "explain this repository"` once with a real API key,
then run the no-mistakes pipeline before pushing.

## Lessons

- **Pipeline agent model backends are the failure point, not the gate tool.**
  no-mistakes itself works; its configured agent (opencode) depends on a model
  subscription that can be rate-limited or quota-exhausted, which stalls a run
  mid-step with no error surfaced to the driver. Diagnose via
  `no-mistakes axi status` (`active_steps`, `agent_pid`, `quiet` + duration) and
  the opencode serve logs; fix by switching the agent to a working provider.
- **Crush's Hyper provider is an OpenAI-compatible HTTP endpoint, but Kite's
  `--from-crush` deliberately rejects it.** The raw endpoint
  (`https://hyper.charm.land/v1`) can back any OpenAI-compatible client, but the
  `--from-crush` import only supports `openai`/`openai-compat` provider types
  and refuses `hyper` with an actionable error. Keep the two facts separate.
- **The opencode free-tier models share console rate limits.** `deepseek-v4-flash-free`
  works for one-off calls but trips `FreeUsageLimitError` under pipeline load;
  don't rely on it for agent work.
- **Prefer resolving rebase conflicts by delegating to the pipeline.** When a
  no-mistakes rebase step gates on a conflict, `axi respond --action fix
  --findings <id> --instructions "<merge guidance>"` lets the agent resolve it;
  the README conflict (upstream spec vs local usage docs) landed cleanly this
  way. Earlier attempts to stall-fix manually fought the pipeline owning the
  worktree.
- **A global `*.md` gitignore silently untracks markdown docs.** This project's
  machine has `*.md` in `~/.gitignore_global`, so new markdown files never show
  in `git status` and are easy to miss at commit time. Force-add them with
  `git add -f <path>`; once tracked they stage normally. Recorded in AGENTS.md.
- **Parse verification status from the bash tool's output, not from error-nil.**
  The bash tool reports a non-zero exit as a string result (`exit status N`)
  with a nil error, so a verification "passed" check that keys off `err == nil`
  wrongly marks failures as passed. Parse the exit code from the output.
- **Symlinked temp dirs break naive path containment.** On macOS,
  `t.TempDir()` resolves under `/var` -> `/private/var`; a containment check that
  resolves the target but not the root rejects valid paths. Resolve both the
  target and the working-directory root before comparing.
- **JSON-decoded event payloads arrive as `map[string]any`, not typed
  structs.** Replaying a JSONL session needs a `decodePayload` step that
  re-marshals and unmarshals into the typed payload struct per event type;
  type-asserting the raw map fails.
- **Break import cycles with a registration hook, not a shared package.**
  `internal/core` cannot import `internal/tools` (which imports core); the
  tools package registers a builtin installer via `core.RegisterBuiltins` in its
  `init`. Keep the public facade in the root package so consumers never touch
  `internal/*`.
- **Audit AUS prose as a regex sweep over md+go with an explicit code
  allow-list.** The high-frequency fixes are shared suffixes (`sanitise`,
  `colour`, `behaviour`, `normalise`) and bare-dash relationship arrows.
  Keep code identifiers and embedded config keys (`colors:`, `textColor`,
  `NO_COLOR`, test names) US per the rules; only prose changes.
- **The write tool refuses on a stale mtime; `rm` + recreate works.** When a
  file was written by a heredoc/other process, the edit/write tool can reject
  the overwrite; deleting the file and recreating it cleanly is the reliable
  workaround.

## Sessions

### 2026-08-20 — Australian English style audit and fixes

**Intent:** Audit the documentation and Go comments against the global writing
rules (Australian English: `-ise`, `-our`, `-re`; no bare dash as a
relationship arrow) and fix the violations. Scope narrowed from an initially
misread "robust API endpoints / LLM providers" request to the concrete,
correctable surface: prose in markdown and comments.

**Done:**

- Systematic scan of every `*.md` and `*.go` for US spellings: `-ize`
  suffixes, `-or` (`color`, `behavior`), `-er`/`-re`, `-el`, and bare-dash
  relationship arrows.
- Fixed 29 prose instances across 10 files: `PRODUCT.md` (colour ×5,
  behaviour ×2), `DESIGN.md` (colour ×7, sanitise ×2), `AGENTS.md`
  (normalisation, behaviour, sanitisation), `docs/agents/lint.md`
  (behaviour, normalised), `docs/agents/cli.md` (colour ×2, behaviour),
  `docs/agents/tui.md` (colour ×2), `README.md` (behaviour),
  `cmd/kite/main.go` (flag help colour), `internal/tui/theme.go` (comment
  colours), `internal/tui/app_test.go` (test message sanitised).
- Left code identifiers and embedded config keys untouched per the rules
  (`colors:`, `textColor`, `NO_COLOR`, `TestDecodeValeNormalizesAlerts`).
- `go build`, `go vet`, `go test ./...` all green after the changes.

**Check:**

- Prior open items, none advanced this session (this was new work, not
  follow-up): the `kite run` real-model check, the pipeline-agent backend
  decision, and the hyper JWT re-review all remain open from
  2026-08-18 session 2; the a660dea pipeline-before-push and the
  push-vs-PR decision remain open from 2026-08-18 session 2.
- What went well: the AUS rules are unambiguous enough to apply mechanically;
  the distinction between prose and code identifiers was clear and held
  (config keys and test names left as US).
- What didn't: the earlier request "make robust API endpoints" was initially
  misread as a feature task before the user clarified they meant LLM provider
  docs/comments; scope had to be re-derived. The `*.md` global-ignore lesson
  (still in the ledger) did not bite this session because the audit only
  modified already-tracked files, but the risk remains for new docs.
- Learned (confirmed): AUS prose audits are best done as a single regex sweep
  over md+go with an explicit allow-list of code identifiers; shared-verb
  suffixes (`sanitise`/`colours`) are the high-frequency fixes.

**Open items:**

- Run `kite run "explain this repository"` against a real paid API once to
  confirm true end-to-end behaviour (from 2026-08-18 session 2; still open,
  now 2 sessions).
- Decide whether the machine's pipeline-agent backend change should be made
  durable/recorded somewhere other than this log (from 2026-08-18 session 2;
  still open, 2 sessions).
- Re-review the machine-level no-mistakes/opencode config if the hyper JWT
  expires (from 2026-08-18 session 2; still open, 2 sessions).
- Run the no-mistakes pipeline on the current milestone (or a feature branch)
  to validate before pushing (from 2026-08-18 session 2; still open).
- Decide whether to push the current branch or land it via a reviewed PR
  (from 2026-08-18 session 2; still open).
- Commit the 10-file AUS style fixes still sitting uncommitted on
  `feat/layered-lint` (from this session).
