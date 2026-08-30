# Layered lint

`kite lint` combines reproducible repository hygiene with optional prose and
model-assisted style review. Each layer is identified on every finding.

```sh
kite lint
kite lint -json docs README.md
kite lint -vale docs
kite lint -vale -llm -from-crush docs
```

## Layers

### Deterministic

The built-in layer is always first, requires no network or external program,
and scans Git-tracked plus non-ignored untracked files in sorted order. For an
explicit path it scans that file or directory. Binary files, symlinks, and
files larger than 1 MiB are reported as skipped.

| Rule | Meaning |
| --- | --- |
| `KITE001` | Trailing whitespace |
| `KITE002` | Missing final newline |
| `KITE003` | Complete unresolved merge-conflict marker set |
| `KITE004` | Invalid UTF-8 text |
| `KITE005` | Line longer than `-max-line` characters |
| `KITE006` | Bidirectional or directional Unicode control |
| `KITE007` | Unexpected text control character |

The CLI default is `-max-line 120`; set `-max-line 0` to disable that rule.

### Vale

`-vale` runs an installed Vale executable as a deterministic prose layer:

```sh
kite lint -vale docs
kite lint -vale -vale-bin /opt/bin/vale docs
```

Kite invokes `vale --output=JSON`, so the repository's `.vale.ini`, styles,
vocabulary, markup parsing, and ignore behaviour remain owned by Vale. Vale
`error` and `warning` alerts affect Kite's exit code; Vale `suggestion` alerts
are normalised to `info`. The Vale check name is preserved as the rule ID.
Vale is optional and is not downloaded or added as a Go dependency.

### LLM review

`-llm` sends a bounded source snapshot to the configured OpenAI-compatible
provider and requests strict JSON findings about clarity, consistency,
documentation, naming, and maintainability. Files and content are sorted,
the combined source payload is capped at 128 KiB, no tools are advertised,
and returned paths and line numbers are validated against the supplied input.

Model findings use layer `llm`, rule `LLM001`, and severity `warning` or
`info`. They are advisory and do not change the exit code by default. Add
`-llm-strict` when a workflow intentionally wants model warnings to fail.

Using `-llm` sends selected repository source to the configured provider. Do
not enable it for material that the provider is not permitted to receive.

## Output and exit codes

Human output follows `path:line:column` diagnostics. `-json` emits the
versioned `kite.lint/v1` contract documented by
[`docs/schemas/v1/lint.json`](../schemas/v1/lint.json). Findings are sorted by
path, line, column, layer, and rule.

- `0` — no failing deterministic or Vale findings
- `1` — lint findings, Vale/provider execution failure, or interrupted review
- `2` — invalid flags or configuration

LLM warnings affect exit code `1` only with `-llm-strict`.
