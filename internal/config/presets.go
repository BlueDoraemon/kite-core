package config

// Preset is a known provider with its API endpoint and a sensible default
// model. Endpoints are OpenAI-compatible chat completions roots; Kite speaks
// one wire format, so presets only remove typing, never add behaviour.
type Preset struct {
	// Name is the stable identifier used by `kite setup -provider <name>`.
	Name string
	// Label is the display name shown in the wizard.
	Label string
	// BaseURL is the OpenAI-compatible API root.
	BaseURL string
	// Model is the default model identifier when the user chooses none.
	Model string
	// NeedsKey is false for local servers that accept requests without a
	// credential.
	NeedsKey bool
	// KeyHint tells the user where to obtain a credential.
	KeyHint string
}

// Presets lists the providers Kite recognises during setup. The order is the
// menu order: hosted providers first, local last.
var Presets = []Preset{
	{Name: "openai", Label: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", NeedsKey: true, KeyHint: "https://platform.openai.com/api-keys"},
	{Name: "groq", Label: "Groq", BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile", NeedsKey: true, KeyHint: "https://console.groq.com/keys"},
	{Name: "openrouter", Label: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", Model: "openai/gpt-4o-mini", NeedsKey: true, KeyHint: "https://openrouter.ai/keys"},
	{Name: "moonshot", Label: "Moonshot (Kimi)", BaseURL: "https://api.moonshot.ai/v1", Model: "kimi-k2-0905-preview", NeedsKey: true, KeyHint: "https://platform.moonshot.ai/console/api-keys"},
	{Name: "deepseek", Label: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat", NeedsKey: true, KeyHint: "https://platform.deepseek.com/api_keys"},
	{Name: "ollama", Label: "Ollama (local)", BaseURL: "http://127.0.0.1:11434/v1", Model: "qwen3:8b", NeedsKey: false, KeyHint: "run `ollama serve` and pull a model first"},
	{Name: "custom", Label: "Custom OpenAI-compatible endpoint", BaseURL: "", Model: "", NeedsKey: false, KeyHint: "any server speaking /v1/chat/completions"},
}

// PresetByName returns the preset with the given name, or nil.
func PresetByName(name string) *Preset {
	for i := range Presets {
		if Presets[i].Name == name {
			return &Presets[i]
		}
	}
	return nil
}
