# `pr-review` Documentation

Welcome to the comprehensive documentation for **`pr-review`**, an extensible CLI and interactive Terminal User Interface (TUI) tool for AI-powered code reviews across **GitHub**, **GitLab**, and **Gerrit**.

---

## 📑 Documentation Index

1. [🖥️ Interactive TUI Dashboard](tui.md) — How to browse pull requests, inspect syntax-highlighted diffs, run reviews, compare multiple AI models side-by-side, and post reviews.
2. [🔮 AI Configuration Wizard](wizard.md) — Step-by-step guide to using the interactive TUI wizard (`pr-review wizard`) to configure local and cloud AI providers, servers, and models.
3. [🤖 AI Providers & Endpoints](ai_engines.md) — Configuring Ollama, vLLM, llama.cpp, Anthropic Claude, Google Gemini, OpenAI, and custom GPU inference clusters.
4. [🛠️ CLI Reference](cli.md) — Non-interactive command-line usage for automated CI/CD pipelines, scripting, listing, and cleaning cached reviews.

---

## 🌟 Key Capabilities

- **Unified Multi-Git Provider Support**: Work seamlessly across GitHub Pull Requests, GitLab Merge Requests, and Gerrit Changes.
- **Many-to-Many AI Model & Endpoint Architecture**: Connect to multiple servers running Ollama, vLLM, llama.cpp, Anthropic Claude, Gemini, and OpenAI with multiple models per provider.
- **Interactive TUI Dashboard**: Full-screen terminal dashboard built with Bubble Tea and Lipgloss with vim-friendly navigation.
- **Side-by-Side Review Comparison**: Generate reviews from different AI models and compare them side-by-side in real-time.
- **Project Context Awareness**: Pass project descriptions to AI engines to give domain-specific context for code reviews.
- **Local Review Caching**: AI reviews are saved locally as formatted Markdown files, letting you inspect, edit, or regenerate before posting.
- **Interactive TUI Setup Wizard**: Easily configure new providers, endpoints, and models without manually editing YAML.

---

## 📸 Screenshots Overview

| Feature | Screenshot |
| :--- | :--- |
| **Interactive TUI Dashboard** | ![TUI Dashboard](images/tui_dashboard.png) |
| **AI Review Engine Selector** | ![AI Selector](images/ai_selector_modal.png) |
| **Side-by-Side Comparison** | ![Comparison](images/side_by_side_comparison.png) |
| **AI Configuration Wizard** | ![TUI Wizard](images/tui_wizard.png) |
| **Review Version Selector** | ![Review Versions](images/review_versions_modal.png) |

---

## 🚀 Quick Start

```bash
# 1. Launch the interactive TUI configuration wizard
pr-review wizard

# 2. Launch the interactive dashboard
pr-review

# 3. Or run reviews directly from the command line
pr-review review <project-id> <pr-number> --provider vllm
```
