# Milestone 5 – CLI-First UI Shell

## Goal
Provide a UI that constructs and runs CLI commands without owning logic.

## Scope
- TUI (Bubble Tea or equivalent)
- Command builder
- Output viewer

## Steps
1. Define stable CLI JSON outputs.
2. Implement TUI panels:
   - Systems & completion %
   - Search/filter releases
   - Build command preview
3. Execute commands and stream output.
4. Display plan JSON visually.
5. Allow user to approve/reject actions before execution.

## Acceptance Criteria
- UI never mutates state directly
- All actions traceable to CLI commands
- UI usable entirely via keyboard

