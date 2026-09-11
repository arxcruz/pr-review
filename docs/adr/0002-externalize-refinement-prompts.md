# Externalize refinement prompts to an editable markdown file

The Frontier and Decomposition system prompts, along with the guidelines text injected into both, were hardcoded Go string constants in `pkg/refine/prompt.go`, so a user could not adjust how `jira-refine` reasons about their tickets without rebuilding the binary. We moved both system prompts plus a shared Guidelines section into a single `jira-refine.md`, embedded (`go:embed`) as the built-in default, overridable via `--prompt-file`, and otherwise auto-written to `~/.config/jira-refine/jira-refine.md` on first use (with a message telling the user where it went) so the file is always present and editable going forward. The ticket/round data rendering (`formatTicketPromptContext`) and the JSON-retry-feedback text stay in Go: they're structural plumbing, not wording a user tunes per project.

## Considered Options

- **Keep prompts as Go constants** (status quo): rejected — the whole point was letting users edit refinement behavior without a rebuild.
- **External file only, no `go:embed`, hard error if missing**: rejected — breaks for a `go install`ed binary run outside a repo checkout, and a hard error on first run is worse UX than "here's a default, and here's where I put it for you."
- **Separate files per prompt** (`frontier.md`, `decomposition.md`): rejected in favor of one `jira-refine.md` with sections, matching what was asked for and keeping one file to find/edit/diff.
- **Guidelines kept as a separate mechanism** from the prompt file: rejected — guidelines are exactly the kind of wording a user wants to tune alongside the prompts, so they live in the same file as a shared `## Guidelines` section referenced by both prompt sections.
