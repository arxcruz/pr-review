# 🛠️ Command-Line Interface (CLI) Reference

`pr-review` provides a full suite of CLI subcommands suitable for scripting, automation, CI/CD pipelines, and fast terminal operations.

---

## 1. `pr-review list`
List and filter open pull requests and merge requests across configured repositories.

```bash
# List all open PRs across all configured projects
pr-review list

# Filter by project alias
pr-review list --project backend

# Filter by author
pr-review list --author alice,bob

# Filter by label
pr-review list --label "needs-review,backend"

# Combine filters
pr-review list -p backend -a alice -l bug
```

---

## 2. `pr-review diff`
Inspect the formatted git diff of a specific pull request without launching the TUI.

```bash
pr-review diff <project-id> <pr-number>

# Example:
pr-review diff backend 42
```

---

## 3. `pr-review review`
Execute an AI review for a pull request, inspect existing reviews, or post comments to GitHub/GitLab.

```bash
# Generate AI review and save locally (e.g. reviews/backend-42-ollama-qwen.md)
pr-review review backend 42

# Generate review with explicit provider and model
pr-review review backend 42 --provider vllm --model Qwen/Qwen2.5-Coder-32B-Instruct

# Review using custom endpoint (e.g. gpu1-vllm)
pr-review review backend 42 --provider gpu1-vllm

# Review using cloud APIs
pr-review review backend 42 --provider anthropic --model claude-3-7-sonnet-20250219
pr-review review backend 42 --provider gemini --model gemini-2.5-flash

# Post existing cached review to GitHub/GitLab comment:
pr-review review backend 42 --post

# Force regeneration with AI even if cached file exists:
pr-review review backend 42 --post --force

# Use a custom review guidelines prompt file:
pr-review review backend 42 --review-file my-guidelines.md
```

---

## 4. `pr-review clean`
Delete cached AI review markdown files.

```bash
# Delete all reviews for PR #42 (with confirmation prompt)
pr-review clean backend 42

# Delete without prompt
pr-review clean backend 42 --force

# Delete all reviews for a project
pr-review clean backend --all

# Delete all cached reviews across all projects
pr-review clean --all

# Delete a specific review file
pr-review clean --file reviews/backend-42-ollama-qwen.md
```

---

## 5. `pr-review config`
Inspect, create, or manage configurations.

```bash
# Launch interactive TUI configuration wizard
pr-review wizard
pr-review config wizard

# Display active configuration with secrets masked
pr-review config show

# Generate a starter template configuration file
pr-review config init
pr-review config init --global
```
