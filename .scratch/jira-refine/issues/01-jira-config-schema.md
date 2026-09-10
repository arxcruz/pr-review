# 01: Jira Configuration Schema & Loader

**What to build:** Extend repository configuration parsing to read Jira server connection details, origin project key, team delivery project mappings, and multi-repo documentation paths from `~/.pr-review.yaml`.

**Blocked by:** None (can start immediately)

**Status:** completed

- [x] Configuration loader reads Jira URL, auth credentials (token or PAT), and default origin project
- [x] Configuration loader maps team names to target delivery project keys and issue types
- [x] Configuration loader parses list of local multi-repo documentation paths
- [x] Unit tests verify config loading, missing field validation, and defaults
