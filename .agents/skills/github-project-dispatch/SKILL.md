---
name: github-project-dispatch
description: 'Dispatch GitHub Project work into separate Codex tasks: implementation issues from "Ready for agent" and requirements work for issues or draft items from "Need requirements". Use when the user asks to start, assign, fan out, or create one Codex task per eligible Project card, especially in mishankov''s todai project 8. Do not implement cards or write requirements in the dispatcher; delegate each card to its dedicated task.'
---

# GitHub Project Dispatch

Create one user-visible Codex task per eligible GitHub Project card. Treat the Project status as the claim lock. Leave implementation to `$github-project-worker`; let dedicated requirements tasks refine issues or draft items and submit them for requirements review.

## Defaults

Use these values unless the user supplies another Project:

- Project: `https://github.com/users/mishankov/projects/8`
- owner: `mishankov`
- project number: `8`
- repository: `mishankov/todai`
- implementation source status: `Ready for agent`
- requirements source status: `Need requirements`
- claimed status: `In Progress`
- requirements review status: `Requirements review`

Discover project, field, option, and item node IDs at runtime. Never rely on copied node IDs.

## Dispatch workflow

1. Require an explicit request to create or start separate Codex tasks. A request to inspect, count, or discuss ready cards is read-only.
2. Verify `gh auth status` and ensure the active token can read repositories and write Projects.
3. Read the Project and its fields with `gh project view` and `gh project field-list`. Verify the exact `Status`, `Ready for agent`, `Need requirements`, `In Progress`, and `Requirements review` names before mutating anything.
4. List all items with `gh project item-list <number> --owner <owner> --limit 100 --format json`. Partition eligible items into:
   - issue cards whose current status is exactly `Ready for agent`;
   - task cards whose current status is exactly `Need requirements`, whether the card contains a repository issue or a project draft.
5. Resolve the Codex saved project with the thread-management project-list tool. Match the repository or local checkout; do not create projectless coding tasks.
6. Process eligible implementation issues one at a time:
   - Re-read the card immediately before claiming it. Skip it if it is no longer `Ready for agent`.
   - Set only that item's `Status` field to `In Progress` with `gh project item-edit`.
   - Create a separate Codex task in a new worktree based on the saved project's default branch. Do not inherit the dispatcher's working tree or detached HEAD.
   - Put the implementation worker prompt below into the new task and substitute all placeholders.
   - If task creation fails synchronously, return the item to `Ready for agent` and report the error. Do not roll back a successfully queued worktree.
   - When a thread ID is available, title it `[GH #<issue-number>] <issue-title>`.
7. Process eligible requirements cards one at a time:
   - Re-read the card immediately before claiming it. Skip it if it is no longer `Need requirements`.
   - Set only that item's `Status` field to `In Progress` with `gh project item-edit`. This claim prevents duplicate requirements tasks.
   - Create a separate Codex task associated with the saved project. A coding worktree is not required because this task must not implement the card.
   - Put the requirements worker prompt below into the new task and substitute all placeholders, including the content-specific reference and body update command.
   - If task creation fails synchronously, return the item to `Need requirements` and report the error.
   - When a thread ID is available, title it `[Requirements] <item-title>`.
8. Report created, skipped, and failed cards. Include the issue URL for repository issues and the Project URL plus item title for drafts. Emit the app's created-thread directive for every successfully created thread or queued worktree.

Do not implement cards, write requirements, wait for workers to finish, or perform worker-owned status transitions in the dispatcher task.

## Implementation worker prompt

Use a prompt containing all durable rules so later follow-ups in the worker task preserve the workflow:

```text
Use $github-project-worker for exactly this card:
- Project: <project-url> (owner <owner>, number <project-number>)
- Project item ID: <project-item-id>
- Issue: <issue-url>
- Repository: <owner/repo>

The dispatcher claimed the card by moving it to In Progress. Work only on this issue in this dedicated worktree. Implement the acceptance criteria, run the relevant checks, publish a PR that is ready for review, and keep the Project status synchronized. Before any later review-fix coding, move Review -> In Progress; after pushing verified fixes, move it back to Review. Move it to Done only after the PR is confirmed merged. For every future implementation or review-fix turn in this task, continue to apply $github-project-worker.
```

## Requirements worker prompt

Use a prompt containing the entire requirements workflow so clarifications and later follow-ups remain in the dedicated task:

```text
Develop requirements for exactly this GitHub Project card:
- Project: <project-url> (owner <owner>, number <project-number>)
- Project item ID: <project-item-id>
- Content: <content-reference>
- Title: <item-title>
- Repository for context: <owner/repo>

The dispatcher claimed the card by moving it from Need requirements to In Progress. Work only on requirements for this card; do not implement it. Preserve its content type: keep an existing repository issue as that same issue, and do not convert a project draft to a repository issue.

Inspect the card body, the repository, and relevant linked context before writing. Produce requirements that are specific enough for a later implementation agent, including the goal and context, scope, relevant constraints, acceptance criteria, and explicit out-of-scope decisions when useful. Preserve useful facts from the original body.

If a material product or behavior choice cannot be inferred safely, ask the user focused clarification questions in this task. Keep the Project item In Progress while any blocking question remains. When the requirements are complete and no blocking questions remain, update the card body with `<body-update-command>` and move only this item's Status to Requirements review. For a repository issue, use `gh issue edit <issue-number> --repo <owner/repo> --body-file <file>`. For a project draft, use `gh project item-edit --id <project-item-id> --body <requirements>`. Discover the Project, Status field, and option node IDs at runtime; never copy node IDs from the prompt. Verify the updated body and final status before reporting completion.

For every future clarification or requirements-editing turn in this task, continue this same workflow. Never move the item to Requirements review merely because a draft was produced; move it only when it is ready for the user to review.
```

## Safety and idempotency

- Use status, not titles or labels, as the eligibility gate.
- Never create a second task for an item already in `In Progress`, `Requirements review`, `Review`, or `Done`.
- Treat `Need requirements` as the requirements eligibility gate, not the card's issue-versus-draft content type. Send both repository issues and project drafts to dedicated requirements tasks, never to `$github-project-worker`.
- Never dispatch `Backlog`, pull-request, redacted, or other ineligible cards without explicit user direction.
- Never change the status of one item because another item's task creation failed.
- Keep a claimed item `In Progress` once its Codex task is successfully created; its dedicated worker owns subsequent transitions.
- Requirements workers may edit only their assigned issue or draft title/body and Project status. They must not implement code or alter unrelated items.
