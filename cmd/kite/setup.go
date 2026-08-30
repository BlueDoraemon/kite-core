package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/BlueDoraemon/kite-core/internal/config"
)

// cmdSetup walks through provider configuration and writes the user config
// file. Interactive by default; supplying -provider (or -base-url) runs it
// non-interactively for scripting. The connection is probed before anything
// is saved so a bad credential never reaches disk silently.
func cmdSetup(args []string) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		provider = fs.String("provider", "", "preset name: openai, groq, openrouter, moonshot, deepseek, ollama, custom")
		baseURL  = fs.String("base-url", "", "API base URL (implies non-interactive with -model)")
		model    = fs.String("model", "", "model identifier")
		apiKey   = fs.String("api-key", "", "credential to store inline (prefer -key-env)")
		keyEnv   = fs.String("key-env", "", "environment variable holding the credential")
		force    = fs.Bool("force", false, "overwrite an existing config file without asking")
		skipTest = fs.Bool("skip-test", false, "skip the connection probe")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "kite: setup takes no positional arguments")
		return 2
	}

	interactive := *provider == "" && *baseURL == ""
	in := bufio.NewReader(os.Stdin)
	out := os.Stdout

	existing, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 2
	}
	if existing.Exists {
		fmt.Fprintf(out, "Existing config found at %s\n", existing.Path)
		if !*force {
			if interactive {
				fmt.Fprint(out, "Replace it? [y/N] ")
				if answer, _ := in.ReadString('\n'); !yes(answer) {
					fmt.Fprintln(out, "kept existing config")
					return 0
				}
			} else {
				fmt.Fprintln(os.Stderr, "kite: config file exists; pass -force to replace it")
				return 2
			}
		}
	}

	cfg := &config.File{Version: config.Contract}

	if interactive {
		printPresetMenu(out)
		name := ask(out, in, "Provider", "openai")
		p := config.PresetByName(name)
		if p == nil {
			fmt.Fprintf(os.Stderr, "kite: unknown provider %q\n", name)
			return 2
		}
		*provider = p.Name
		fmt.Fprint(out, "Base URL"+promptSuffix(p.BaseURL)+": ")
		if v, _ := in.ReadString('\n'); strings.TrimSpace(v) != "" {
			*baseURL = strings.TrimSpace(v)
		} else {
			*baseURL = p.BaseURL
		}
		fmt.Fprint(out, "Model"+promptSuffix(p.Model)+": ")
		if v, _ := in.ReadString('\n'); strings.TrimSpace(v) != "" {
			*model = strings.TrimSpace(v)
		} else {
			*model = p.Model
		}
		// Every preset gets the credential question: local servers usually
		// need none, and custom endpoints frequently do.
		suggestion := envSuggestion(p)
		fmt.Fprint(out, "Environment variable holding your API key"+promptSuffix(suggestion)+" (empty for none): ")
		v, _ := in.ReadString('\n')
		*keyEnv = strings.TrimSpace(v)
		if *keyEnv == "" && p.NeedsKey {
			fmt.Fprint(out, "Paste the API key to store inline (empty to skip): ")
			v, _ := in.ReadString('\n')
			*apiKey = strings.TrimSpace(v)
		}
	} else {
		if *provider != "" {
			p := config.PresetByName(*provider)
			if p == nil {
				names := make([]string, 0, len(config.Presets))
				for _, preset := range config.Presets {
					names = append(names, preset.Name)
				}
				fmt.Fprintf(os.Stderr, "kite: unknown provider %q (choose: %s)\n", *provider, strings.Join(names, ", "))
				return 2
			}
			if *baseURL == "" {
				*baseURL = p.BaseURL
			}
			if *model == "" {
				*model = p.Model
			}
		}
		if *baseURL == "" {
			fmt.Fprintln(os.Stderr, "kite: non-interactive setup needs -provider or -base-url")
			return 2
		}
		if *model == "" {
			*model = config.DefaultModel
		}
	}

	cfg.BaseURL = *baseURL
	cfg.Model = *model
	cfg.KeyEnv = *keyEnv
	cfg.APIKey = *apiKey

	resolvedKey := apiKeyFor(cfg)
	if !*skipTest {
		fmt.Fprintf(out, "Testing %s with model %s...\n", cfg.BaseURL, cfg.Model)
		if err := config.TestConnection(context.Background(), cfg.BaseURL, resolvedKey, cfg.Model); err != nil {
			fmt.Fprintf(os.Stderr, "kite: connection test failed: %v\n", err)
			if interactive {
				fmt.Fprint(out, "Save anyway? [y/N] ")
				if answer, _ := in.ReadString('\n'); !yes(answer) {
					fmt.Fprintln(out, "nothing saved")
					return 1
				}
			} else {
				return 1
			}
		}
		fmt.Fprintln(out, "Connection OK")
	}

	path := existing.Path
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kite:", err)
			return 2
		}
	}
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	fmt.Fprintf(out, "Saved %s\n", path)
	fmt.Fprintf(out, "Next: kite run \"explain this repository\"\n")
	if cfg.KeyEnv != "" && os.Getenv(cfg.KeyEnv) == "" {
		fmt.Fprintf(out, "Note: export %s (or set KITE_API_KEY) before running.\n", cfg.KeyEnv)
	}
	return 0
}

func printPresetMenu(out *os.File) {
	fmt.Fprintln(out, "Providers:")
	for _, p := range config.Presets {
		note := "key required"
		if !p.NeedsKey {
			note = "no key needed"
		}
		fmt.Fprintf(out, "  %-11s %-36s default model: %-24s (%s)\n", p.Name, p.Label, p.Model, note)
	}
}

func ask(out *os.File, in *bufio.Reader, label, def string) string {
	fmt.Fprintf(out, "%s [%s]: ", label, def)
	line, _ := in.ReadString('\n')
	if v := strings.TrimSpace(line); v != "" {
		return v
	}
	return def
}

func promptSuffix(def string) string {
	if def == "" {
		return ""
	}
	return " [" + def + "]"
}

func yes(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}

func envSuggestion(p *config.Preset) string {
	switch p.Name {
	case "openai":
		return "OPENAI_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "moonshot":
		return "MOONSHOT_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	default:
		return ""
	}
}

func apiKeyFor(cfg *config.File) string {
	if cfg.KeyEnv != "" {
		if v := os.Getenv(cfg.KeyEnv); v != "" {
			return v
		}
	}
	return cfg.APIKey
}
