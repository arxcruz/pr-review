# Spec: `jira-refine`

Strategic Jira ticket refinement tool integrated into the `pr-review` repository sharing existing multi-LLM abstractions (`pkg/ai`).

## Summary
Interactive tool that takes a Strategic Ticket from an Origin Project (e.g. `ORIGIN`), ingests multi-repo architectural context, conducts a frontier-based interview (`grill-with-docs` style), decomposes the initiative into a Decomposition Tree of Epics and Tasks routed across team Delivery Projects, persists offline session snapshots, and synchronizes the created items to Jira with cross-project issue links.

## Architectural Decisions
- [docs/adr/0001-jira-refine-architecture.md](../../docs/adr/0001-jira-refine-architecture.md)
- Domain vocabulary: [CONTEXT.md](../../CONTEXT.md)
