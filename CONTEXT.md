# Ticket Refinement

Interactive breakdown of strategic initiatives into structured, actionable delivery items.

## Language

**Strategic Ticket**:
A high-level Jira issue defining a broad business goal or multi-team initiative.
_Avoid_: Big task, parent ticket, initiative

**Origin Project**:
The Jira project containing the Strategic Ticket being refined.
_Avoid_: Source project, parent project

**Delivery Project**:
A team-specific Jira project where decomposed items are tracked and executed.
_Avoid_: Target project, child project

**Project Routing**:
Mapping decomposed items to specific Delivery Projects based on team ownership.
_Avoid_: Project dispatch, destination assignment

**Refinement Session**:
An interactive interview loop where questions are posed to resolve ambiguity and clarify requirements.
_Avoid_: Chat, planning meeting, prompt loop

**Refinement Guidelines**:
User-editable rules that shape how Frontier Questions and the Decomposition Tree are generated (e.g. architectural priorities, what counts as a good question). Stored outside the codebase so a user can tune them without a rebuild.
_Avoid_: Prompt, system prompt, instructions

**Frontier Question**:
An unresolved decision whose prerequisites are satisfied, presented to the user in a round.
_Avoid_: Prompt, clarification, inquiry

**Decomposition Tree**:
A hierarchical graph mapping a strategic ticket into epics, stories, and tasks with dependency edges.
_Avoid_: Task list, breakdown, subtask list

**Session Snapshot**:
Persisted state of a refinement session enabling offline review, pause/resume, and deferred synchronization.
_Avoid_: Draft, save file, cache

**Context Note**:
Free-text background supplied by the user before a Refinement Session begins, used to compensate for a sparse Strategic Ticket description.
_Avoid_: Requirement, comment, annotation
