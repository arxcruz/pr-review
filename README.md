# pr-review 🤖

`pr-review` is an interactive TUI and CLI tool written in Go to list, inspect, filter, and review **GitHub Pull Requests**, **GitLab Merge Requests**, and **Gerrit Changes** using AI.

It supports local AI models (via **Ollama** or any **OpenAI-compatible server** like LM Studio / vLLM) as well as cloud providers like **Anthropic Claude**, **Google Gemini**, and **OpenAI**.

---

## ✨ Features

- **Interactive Terminal UI (TUI)**: Beautiful Bubble Tea dashboard with real-time PR browsing, color-coded diff inspector, cached review manager, and keyboard shortcuts.
- **Pre-AI Review Cache Checking**: Automatically detects and displays previously generated reviews (`reviews_dir` or `<repo>-<number>.md`) before making AI calls.
- **Multi-Platform Git Support**: Works seamlessly with:
  - **GitHub** (`github.com` & GitHub Enterprise)
  - **GitLab** (`gitlab.com` & Self-hosted instances)
  - **Gerrit Code Review** (e.g. `review.opendev.org`, `gerrit.googlesource.com`, self-hosted Gerrit)
- **Multiple AI Engines**:
  - **Local AI**: Ollama (`qwen2.5-coder`, `deepseek-coder-v2`, `llama3.1`, etc.) or generic OpenAI-compatible local endpoints.
  - **Anthropic Claude**: `claude-3-7-sonnet-20250219`, `claude-3-5-sonnet-20241022`, etc.
  - **Google Gemini**: `gemini-2.5-flash`, `gemini-2.5-pro`, etc.
  - **OpenAI**: `gpt-4o`, `gpt-4o-mini`, etc.
- **Unified YAML Configuration**: Configure projects, review storage directory (`reviews_dir`), default filters, AI models, and tokens with environment variable expansion (`${GITHUB_TOKEN}`, `${ANTHROPIC_API_KEY}`, `${GERRIT_PASSWORD}`).
- **Line-by-Line & Actionable Reviews**: AI highlights exact line numbers, rationale, and concrete code replacement diffs.
- **Flexible PR/MR/Change Filtering**: Filter by **author(s)**, **label(s)**, review state, or specific projects both via config defaults, live TUI search, and CLI flags.
- **Direct Feedback**: Option to post AI review comments directly back to the pull request / merge request / Gerrit change (`--post` in CLI, or `P` in TUI).

---

## 📚 Documentation

For in-depth guides, architecture diagrams, and walkthroughs, check out the [`docs/`](docs/README.md) directory:

- 🖥️ **[TUI Dashboard Guide](docs/tui.md)** — Layout, navigation, side-by-side comparison, and review posting.
- 🔮 **[AI Configuration Wizard Guide](docs/wizard.md)** — Step-by-step interactive setup walkthrough.
- 🤖 **[AI Providers & Endpoints](docs/ai_engines.md)** — Multi-model setup, vLLM, Ollama, llama.cpp, Claude, Gemini, OpenAI, and project descriptions.
- 🛠️ **[CLI Reference](docs/cli.md)** — Non-interactive scripting and command-line commands.

---

## 🚀 Quick Start

### 1. Installation

```bash
git clone https://github.com/arxcruz/pr-review.git
cd pr-review
make build
# Binary is built at ./pr-review
```

Or install globally:
```bash
make install
```

### 2. Configuration

Generate a starter configuration file:
```bash
# Create local configuration (.pr-review.yaml)
./pr-review config init

# Or create a global configuration (~/.config/pr-review/config.yaml)
./pr-review config init --global
```

Or copy the example:
```bash
cp config.example.yaml ~/.config/pr-review/config.yaml
```

Sample configuration (`~/.config/pr-review/config.yaml`):

```yaml
default_ai_provider: "ollama"

# Directory where generated review markdown files are stored
reviews_dir: "reviews"

# Optional: custom review guidelines markdown file (defaults to review.md)
# review_file: "review.md"

ai:
  # Method 1: Define multiple models under standard providers
  ollama:
    base_url: "http://localhost:11434"
    models:
      - "qwen2.5-coder:latest"
      - "deepseek-coder-v2:16b"
      - "llama3.3:70b"
    temperature: 0.2

  vllm:
    base_url: "http://localhost:8000/v1"
    model: "Qwen/Qwen2.5-Coder-32B-Instruct"
    temperature: 0.2

  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    models:
      - "claude-3-7-sonnet-20250219"
      - "claude-3-5-haiku-20241022"

  gemini:
    api_key: "${GEMINI_API_KEY}"
    model: "gemini-2.5-flash"

  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4o"

  # Method 2: Define multiple custom endpoints / servers (many-to-many)
  # Perfect if you have multiple servers running Ollama, vLLM, llama.cpp, or remote GPUs
  endpoints:
    - id: "gpu1-vllm"
      name: "GPU 1 (DeepSeek)"
      provider: "vllm"
      base_url: "http://192.168.1.50:8000/v1"
      api_key: "${VLLM_API_KEY}"
      models:
        - "deepseek-ai/DeepSeek-Coder-V2-Instruct"
      temperature: 0.2

    - id: "gpu2-vllm"
      name: "GPU 2 (Qwen 32B)"
      provider: "vllm"
      base_url: "http://192.168.1.51:8000/v1"
      models:
        - "Qwen/Qwen2.5-Coder-32B-Instruct"
      temperature: 0.2

    - id: "remote-ollama"
      name: "Remote Ollama Server"
      provider: "ollama"
      base_url: "http://192.168.1.100:11434"
      models:
        - "qwen2.5-coder:32b"
        - "codellama:70b"

    - id: "local-llamacpp"
      name: "llama.cpp (Local Server)"
      provider: "llamacpp"
      base_url: "http://localhost:8080/v1"
      model: "default"
      temperature: 0.2

git:
  github:
    token: "${GITHUB_TOKEN}"
  gitlab:
    token: "${GITLAB_TOKEN}"

projects:
  - id: "backend"
    provider: "github"
    owner: "myorg"
    repo: "backend"
    description: "Core microservice backend built with Go and gRPC. Follows clean architecture."
    default_filters:
      labels: ["needs-review"]
      authors: []

  - id: "frontend"
    provider: "gitlab"
    project_path: "mygroup/subgroup/frontend"
    description: "React and TypeScript web frontend using Tailwind CSS and Next.js."
    default_filters:
      authors: ["alice"]

# Optional: customize keyboard shortcuts (ideal for Moonlander/Ergodox/custom layouts)
# Accepts single key strings (e.g. "k") or list of strings (e.g. ["k", "up"])
keybindings:
  up: ["k", "up"]                 # Scroll up line-by-line in viewports or PR list
  down: ["j", "down"]             # Scroll down line-by-line in viewports or PR list
  page_up: ["ctrl+b", "pgup"]     # Scroll full page up
  page_down: ["ctrl+f", "pgdown"] # Scroll full page down
  half_page_up: ["ctrl+u"]        # Scroll half page up (vim style)
  half_page_down: ["ctrl+d"]      # Scroll half page down (vim style)
  top: ["g", "home"]              # Jump to top of content
  bottom: ["G", "end"]            # Jump to bottom of content
  switch_pane: ["tab", "shift+tab"] # Toggle focus between PR list and Details pane
  tab_overview: ["1"]             # Switch to PR Overview tab
  tab_diff: ["2", "d"]            # Switch to Diff tab
  tab_review: ["3"]               # Switch to AI Review tab
  run_review: ["r"]               # Generate or re-generate AI review
  post_review: ["P"]              # Post AI review comment to remote PR
  next_project: ["p"]             # Switch to next configured project/repo
  refresh: ["R"]                  # Refresh PR list from git provider
  filter: ["/"]                   # Search / filter PRs in real-time
  select_ai: ["a"]                # Open AI provider & model selector modal
  select_review: ["v"]            # Open modal list of saved AI review versions
  prev_review: ["[", "alt+left"]  # Cycle to previous review version
  next_review: ["]", "alt+right"] # Cycle to next review version
  compare_reviews: ["c"]          # Toggle side-by-side review comparison mode
  delete_review: ["x", "delete"]  # Delete currently viewed cached review
  delete_all_reviews: ["X"]       # Delete all cached reviews for current PR
  help: ["?"]                     # Toggle help popup
  quit: ["q", "ctrl+c"]           # Quit application
```

---

## 🖥️ Interactive Terminal UI (TUI)

Launch the interactive dashboard to browse PRs, scroll through diffs, view cached reviews, and generate AI reviews:

```bash
# Launch TUI directly (default when running without arguments)
pr-review

# Or explicitly:
pr-review tui
```

## 🛠️ CLI Usage

### List Pull Requests & Merge Requests

```bash
# List open PRs across all configured projects
pr-review list

# Filter by author
pr-review list --author alice,bob

# Filter by labels
pr-review list --label "needs-review,backend"

# Filter by specific project / repo
pr-review list --project backend

# Combine filters
pr-review list -p backend -a alice -l bug
```

### Perform AI Code Review

When running a review, the output is displayed on screen and automatically saved to `<repository>-<pr-number>-<provider>-<model>.md` (e.g. `backend-42-ollama-qwen2-5-coder-latest.md`).

```bash
# 1. Review a PR (generates review via AI and saves to file)
pr-review review backend 42

# 2. Inspect/edit the review if desired, then submit to GitHub/GitLab:
# (uses the existing review without calling AI again)
pr-review review backend 42 --post

# Force regeneration with AI even if review markdown file exists:
pr-review review backend 42 --post --force

# Review with custom review guidelines file
pr-review review backend 42 --review-file my-custom-guidelines.md

# Review a GitLab MR using Gemini
pr-review review frontend 15 --provider gemini --model gemini-2.5-flash

# Review using Anthropic Claude and post directly
pr-review review backend 42 --provider anthropic --post

# Review using local Ollama (specifying model)
pr-review review backend 42 --provider ollama --model qwen2.5-coder:latest

# Review using custom configured endpoint (e.g. gpu1-vllm or remote-ollama)
pr-review review backend 42 --provider gpu1-vllm

# Review using vLLM with explicit model
pr-review review backend 42 --provider vllm --model Qwen/Qwen2.5-Coder-32B-Instruct
```

### Clean / Delete Cached Reviews

```bash
# Delete all cached reviews for a specific PR
pr-review clean backend 42

# Delete without confirmation prompt
pr-review clean backend 42 --force

# Delete all reviews for a specific project
pr-review clean backend --all

# Delete all cached reviews across all projects
pr-review clean --all

# Delete a specific review file
pr-review clean --file reviews/backend-42-ollama-qwen.md
```

### Inspect PR Diff

```bash
pr-review diff backend 42
```

### Configure AI Providers & Models (Interactive Wizard)

Easily configure new AI providers, remote servers/endpoints (Ollama, vLLM, llama.cpp, OpenAI, Claude, Gemini), and models with the interactive wizard:

```bash
# Launch interactive AI configuration wizard
pr-review wizard

# Or via the config subcommand:
pr-review config wizard

# Target a specific config file:
pr-review wizard --config /path/to/config.yaml
```

The wizard guides you through:
1. Selecting the provider type (Ollama, vLLM, llama.cpp, Anthropic Claude, Google Gemini, OpenAI).
2. Choosing whether to update the standard provider or create a named custom endpoint.
3. Entering base URL / host (with default suggestions).
4. Entering API key / environment variable (e.g. `${ANTHROPIC_API_KEY}`).
5. Selecting recommended models or specifying custom model identifiers.
6. Setting temperature and default provider preference.
7. Safely persisting the updated configuration to `config.yaml` without expanding environment variable references.

### View Configuration

```bash
# View active configuration with secrets masked
pr-review config show
```

---

## 🧱 Architecture

`pr-review` is designed with modular, decoupled packages separating CLI commands, interactive TUI components, git provider abstractions, and AI review engines:

```
pr-review/
├── cmd/pr-review/            # CLI subcommands & entrypoint (Cobra)
│   ├── main.go               # Main application entrypoint
│   ├── root.go               # Global flags, init, and PersistentPreRunE
│   ├── tui.go                # TUI launcher command (`pr-review tui`)
│   ├── list.go               # List & filter PRs across providers (`pr-review list`)
│   ├── diff.go               # Inspect formatted git diffs (`pr-review diff`)
│   ├── review.go             # Run AI reviews & post comments (`pr-review review`)
│   ├── clean.go              # Delete & manage cached reviews (`pr-review clean`)
│   └── config_cmd.go         # Config show, init, and wizard commands (`pr-review config`)
├── pkg/
│   ├── config/               # YAML loader, env var expansion, endpoints & targets
│   ├── gitprovider/          # Provider interface, GitHub, GitLab, and Gerrit clients
│   ├── ai/                   # AI Engine interface, Ollama, vLLM, llama.cpp, Claude, Gemini, OpenAI
│   ├── review/               # High-level ReviewService orchestrator & Glamour/Lipgloss renderers
│   └── tui/                  # Bubble Tea interactive TUI dashboard & configuration wizard
│       ├── app.go            # Main TUI state model, keyboard event handlers, & lifecycle
│       ├── views.go          # Split-pane rendering, side-by-side comparison, modals, & tables
│       ├── styles.go         # Lipgloss color palette, borders, badges, & typography
│       └── wizard.go         # Step-by-step TUI AI configuration wizard
├── docs/                     # Comprehensive guides & documentation
│   ├── README.md             # Documentation portal & overview
│   ├── tui.md                # Interactive TUI dashboard guide & keybindings
│   ├── wizard.md             # TUI AI configuration wizard guide
│   ├── ai_engines.md         # AI provider & endpoint configuration reference
│   └── cli.md                # Non-interactive CLI command reference
├── review.md                 # Default review guidelines and prompt instructions
└── config.example.yaml       # Sample configuration template
```

---

## 🔮 Roadmap

- [x] Multi-Git provider support (GitHub, GitLab, Gerrit)
- [x] Multi-AI engine support (Ollama, vLLM, llama.cpp, Claude, Gemini, OpenAI)
- [x] Many-to-many endpoint and multi-model configuration
- [x] Interactive Terminal User Interface (TUI) with Bubble Tea & Lipgloss
- [x] Side-by-side multi-model AI review comparison
- [x] Interactive TUI AI configuration wizard (`pr-review wizard`)
- [ ] Inline line-level comments and suggestion threads
- [ ] Bitbucket Cloud and Data Center provider support
- [ ] Automated GitHub Actions / GitLab CI comment bot integration
- [ ] Customizable prompt templates per project or file extension

