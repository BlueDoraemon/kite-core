# Process Log — kite

## Proposals

<!-- fix
id: PF-001
target: skill:no-mistakes
status: open
created: 2026-08-18
-->
### PF-001 no-mistakes skill: add a "pipeline agent model backend" diagnostic to the stalled-run section
**Problem:** Across three runs, the pipeline stalled silently mid-step (rebase,
then review) because the configured agent (opencode) had no working model
backend: `opencode-go` hit its monthly quota (429 `GoUsageLimitError`) and the
free tier hit console rate limits (`FreeUsageLimitError`). The skill's
"Validate and decide" section treats long-running steps and `quiet` markers as
liveness clues to wait out — correct for a working agent, but a model-backend
failure can sit parked for 40+ minutes with no surfaced error. The first time,
the stall was misread as a tooling/harness problem and the run was aborted and
restarted, discarding the pipeline's in-flight work. The second and third times
the right diagnostic was: `no-mistakes axi status` (`active_steps` with empty
`agent_pid`, `last_activity` prefixed `quiet`) plus the opencode serve logs
(ERROR `AI_APICallError` with status 429), then switching the agent to a
working provider and re-running.
**Proposed change:** In the skill's stalled/quiet-step guidance, add a short
checklist to distinguish a working agent from a dead model backend before
deciding to keep waiting: (1) when the run has been parked far beyond
`step_quiet_warning`, check `no-mistakes axi status` for an empty `agent_pid`
and `quiet` `last_activity`, (2) inspect the configured agent's own logs (e.g.
opencode serve logs under `~/.no-mistakes/logs/`) for rate-limit/quota errors
(429, `Rate limit exceeded`, usage-limit messages), (3) if a backend failure is
confirmed, switching the pipeline agent's model provider is the fix, not
waiting; then re-run. One sentence can note that `no-mistakes doctor` only
checks binary availability, not model backend health.
**Why it matters:** Every no-mistakes session on this machine pays ~40 minutes
of silent stall (plus the risk of an unnecessary abort) whenever a model
subscription is exhausted. The instruction change costs one paragraph and
benefits every future session that drives a pipeline; the multiplier is the
number of future pipeline runs, so the first occurrence already justifies it
(obligation 6 exception).

<!-- fix
id: PF-002
target: skill:session-wrapup
status: open
created: 2026-08-18
-->
### PF-002 session-wrapup: check for a global `*.md` gitignore before writing the log
**Problem:** This session's wrap-up nearly missed every new markdown deliverable
(AGENTS.md, docs/agents/*, README). The machine has `*.md` in
`~/.gitignore_global`, so none of the new docs appeared in `git status`; the
user had to flag it before they were force-added with `git add -f`. A wrap-up
that records "what happened" but silently omits untracked docs is an incomplete
record, and the same gotcha will hit any future session that creates markdown.
**Proposed change:** In the "Gather the session's facts" section, add one
checklist item: before writing, run `git config --get core.excludesfile` and,
if a global ignore exists, check it for `*.md` (or other patterns that would
hide the session's outputs); if present, note in the Done list that markdown
was force-added with `git add -f`. One sentence is enough.
**Why it matters:** The cost is a missed deliverable in the project's durable
memory every time a session produces markdown on a machine with a global `*.md`
ignore - the multiplier is the number of future wrap-ups, so the first
occurrence already justifies it (obligation 6 exception).

<!-- fix
id: PF-003
target: skill:session-wrapup
status: open
created: 2026-08-18
-->
### PF-003 session-wrapup: use rm + recreate when the write tool refuses on a stale mtime
**Problem:** During this wrap-up, the write tool refused to overwrite
SESSION_LOG.md and PROCESS_LOG.md because their mtimes predated the read
("modified since it was last read"). This cost several retries before the
`rm` + recreate workaround. The same refusal will recur whenever a log file
was written by a previous process (e.g. a heredoc) and the wrap-up's first write
attempt is rejected.
**Proposed change:** In the "Where the log lives" section, add one line: if the
write/edit tool rejects an overwrite with a "modified since it was last read"
error, `rm` the file and recreate it (the mtime is stale, not the content).
**Why it matters:** Every wrap-up that updates an existing log on this machine
can hit the refusal; the fix is a one-line instruction that removes a
recurring retry loop (obligation 6 exception).

<!-- fix
id: PF-004
target: skill:session-wrapup
status: open
created: 2026-08-20
-->
### PF-004 session-wrapup: run an Australian-English prose sweep before writing the log
**Problem:** This session's wrap-up was preceded by a full AUS style audit (10
files, 29 fixes). The wrap-up itself then wrote new prose into SESSION_LOG.md
and PROCESS_LOG.md with no style check, and a bare-dash relationship arrow
slipped into PF-001's text (fixed later). The log is part of the project's
durable memory; US spellings or bare dashes in it are the same defect the
audit was fixing.
**Proposed change:** In the "Gather the session's facts" section, add one
checklist item: before writing the log, run a quick AUS sweep over the
session's outputs and the log's own new prose (shared suffixes `sanitise`,
`colour`, `behaviour`, `normalise`; bare-dash relationship arrows), and fix
violations in the entry being written. One sentence is enough.
**Why it matters:** Every wrap-up writes prose that becomes the project's
memory; a one-line check keeps that memory consistent with the global AUS
rules across every future session - the multiplier is the number of wrap-ups,
so the first occurrence already justifies it (obligation 6 exception).
