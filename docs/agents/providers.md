# Providers

Kite ships one provider: an OpenAI-compatible chat completions adapter that
streams over Server-Sent Events.

## Configuration

| Variable | Flag | Default | Purpose |
| --- | --- | --- | --- |
| `KITE_API_KEY` | (none) | unset | API key sent as a Bearer token |
| `KITE_BASE_URL` | `-base-url` | `https://api.openai.com/v1` | API base URL |
| `KITE_MODEL` | `-model` | `gpt-4o-mini` | Model identifier |

Without `--from-crush`, explicit flags override Kite environment variables,
which override the user config file, which overrides defaults. With
`--from-crush`, explicit `-base-url` and `-model` flags override imported
values, and `KITE_API_KEY` overrides the imported credential. `KITE_BASE_URL`,
`KITE_MODEL`, and the config file are only consulted when Crush import is not
active.

## Config file

The config file lives at `$XDG_CONFIG_HOME/kite/config.json` (default
`~/.config/kite/config.json`) on Unix and `%APPDATA%\kite\config.json` on
Windows. It follows the additive `kite.config/v1` contract; unknown fields are
ignored and a mismatched `version` is rejected rather than silently misread.

```json
{
  "version": "kite.config/v1",
  "base_url": "https://api.example.com/v1",
  "model": "some-model",
  "key_env": "MY_PROVIDER_KEY",
  "api_key": "optional inline credential"
}
```

- `key_env` names an environment variable holding the credential and takes
  precedence over an inline `api_key`, keeping secrets out of the file.
- Files are written with user-only (`0600`) permissions where supported.
- A malformed or unsupported-version file is a configuration error (exit `2`),
  never silent fallback to defaults.
- When no credential resolves from any layer and the base URL is remote, Kite
  reports actionable guidance instead of attempting the request. Local
  endpoints (`http://localhost` or `http://127.0.0.1`, for example Ollama)
  need no credential.

## Streaming behaviour

The provider sends a streaming chat completions request and parses the SSE
stream:

- Text arrives as `content` deltas.
- Tool calls arrive fragmented and are assembled by index.
- Usage arrives in the final chunk.
- `[DONE]` ends the stream.
- Each chat completion request has a five-minute deadline.

## Errors

Errors are sanitised and structured:

- Non-200 responses carry the HTTP status.
- Malformed SSE chunks carry `malformed_sse`.
- Stream read failures carry `stream`.

There is no automatic retry after streamed output, by design.

## Crush import

`--from-crush` reads the persisted Crush-selected large model, credential, and
cached endpoint without executing crushrc.

Limitations:

- Supports OpenAI, OpenAI-compatible, and Hyper providers (Hyper exposes an
  OpenAI-compatible chat completions endpoint).
- OAuth credentials that are expired or within 5 minutes of expiry are
  rejected.
- Errors are secret-free and actionable.

## See also

- [Troubleshooting](troubleshooting.md)
- [Security](security.md)
