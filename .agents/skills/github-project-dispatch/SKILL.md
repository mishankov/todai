---
name: github-project-dispatch
description: Dispatch GitHub Project cards that are in "Ready for agent" into separate Codex tasks and isolated worktrees. Use when the user asks to start, assign, fan out, or create one Codex task per ready card in a GitHub Project, especially mishankov's todai project 8. Do not use to implement a card; delegate each card to github-project-worker.
---

# GitHub Project Dispatch

Create one user-visible Codex task per eligible GitHub Project card. Treat the Project status as the claim lock and leave implementation to `$github-project-worker`.

## Defaults

Use these values unless the user supplies another Project:

- Project: `https://github.com/users/mishankov/projects/8`
- owner: `mishankov`
- project number: `8`
- repository: `mishankov/todai`
- source status: `Ready for agent`
- claimed status: `In Progress`

Discover project, field, option, and item node IDs at runtime. Never rely on copied node IDs.

## Dispatch workflow

1. Require an explicit request to create or start separate Codex tasks. A request to inspect, count, or discuss ready cards is read-only.
2. Verify `gh auth status` and ensure the active token can read repositories and write Projects.
3. Read the Project and its fields with `gh project view` and `gh project field-list`. Verify the exact `Status`, `Ready for agent`, and `In Progress` names before mutating anything.
4. List all items with `gh project item-list <number> --owner <owner> --limit 100 --format json`. Select only issue cards whose current status is exactly `Ready for agent`.
5. Resolve the Codex saved project with the thread-management project-list tool. Match the repository or local checkout; do not create projectless coding tasks.
6. Process eligible cards one at a time:
   - Re-read the card immediately before claiming it. Skip it if it is no longer `Ready for agent`.
   - Set only that item's `Status` field to `In Progress` with `gh project item-edit`.
   - Create a separate Codex task in a new worktree based on the saved project's default branch. Do not inherit the dispatcher's working tree or detached HEAD.
   - Put the worker prompt below into the new task and substitute all placeholders.
   - If task creation fails synchronously, return the item to `Ready for agent` and report the error. Do not roll back a successfully queued worktree.
   - When a thread ID is available, title it `[GH #<issue-number>] <issue-title>`.
7. Report created, skipped, and failed cards with their issue URLs. Emit the app's created-thread directive for every successfully created thread or queued worktree.

Do not implement cards, wait for workers to finish, or move cards to `Review` in the dispatcher task.

## Worker prompt

Use a prompt containing all durable rules so later follow-ups in the worker task preserve the workflow:

```text
Use $github-project-worker for exactly this card:
- Project: <project-url> (owner <owner>, number <project-number>)
- Project item ID: <project-item-id>
- Issue: <issue-url>
- Repository: <owner/repo>

The dispatcher claimed the card by moving it to In Progress. Work only on this issue in this dedicated worktree. Implement the acceptance criteria, run the relevant checks, publish a PR that is ready for review, and keep the Project status synchronized. Before any later review-fix coding, move Review -> In Progress; after pushing verified fixes, move it back to Review. Move it to Done only after the PR is confirmed merged. For every future implementation or review-fix turn in this task, continue to apply $github-project-worker.
```

## Safety and idempotency

- Use status, not titles or labels, as the eligibility gate.
- Never create a second task for an item already in `In Progress`, `Review`, or `Done`.
- Never dispatch `Backlog`, draft-issue, pull-request, or redacted cards without explicit user direction.
- Never change the status of one item because another item's task creation failed.
- Keep a claimed item `In Progress` once its Codex task is successfully created; the worker owns subsequent transitions.
