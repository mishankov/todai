---
name: github-project-worker
description: Implement exactly one GitHub issue from a dedicated Codex task while synchronizing its GitHub Project status through implementation, pull-request review, revisions, CI fixes, and merge. Use for a card claimed from "Ready for agent", for follow-up changes to its PR, or whenever the card must move between "In Progress", "Review", and "Done", especially in mishankov's todai project 8. Do not use to dispatch multiple cards.
---

# GitHub Project Worker

Own exactly one Project card and its issue, branch, worktree, and PR. Keep GitHub Project status aligned with actual work rather than anticipated work.

## Defaults and identity

Use the Project and issue coordinates from the task prompt. If omitted, default only the Project to `https://github.com/users/mishankov/projects/8` (owner `mishankov`, number `8`) and derive the issue from the prompt or current branch.

Before editing code:

1. Verify `gh auth status` and the repository remote.
2. Read the issue, comments, labels, and acceptance criteria.
3. List Project items and resolve the card by its issue URL. If the supplied Project item ID and URL identify different cards, stop and report the mismatch.
4. Read repository instructions and use any matching repo-local skills.
5. Work only on this issue. Do not inspect or claim other ready cards.

Discover Project and Status node IDs at runtime with `gh project view`, `gh project field-list`, and `gh project item-list`. Never embed stale node IDs in commands.

## State machine

Apply these transitions:

| Event | Required status |
|---|---|
| Worker begins an eligible issue | `In Progress` |
| Worker begins code changes requested during review | `In Progress` |
| Verified implementation or revisions are pushed and a non-draft PR is open | `Review` |
| PR is closed without merge and work remains | `In Progress` |
| PR is confirmed merged | `Done` |

Handle the initial status as follows:

- `Ready for agent`: move to `In Progress`, then begin.
- `In Progress`: continue.
- `Review`: perform read-only review checks without changing status; move to `In Progress` before editing code or other PR content in response to feedback.
- `Backlog`: stop unless the user explicitly authorizes starting a backlog item.
- `Done`: stop unless the user explicitly asks to reopen or correct the completed work.

Change only the current card's `Status` field using `gh project item-edit`.

## Implementation workflow

1. Ensure the card is `In Progress` before the first code edit.
2. Confirm the worktree is isolated. Create or use a branch named `codex/issue-<number>-<short-slug>` unless the task already has an appropriate branch.
3. Implement the issue's acceptance criteria and preserve unrelated user changes.
4. Run checks proportional to the change, including repository-required tests and formatting. Keep the card `In Progress` when blocked or when required checks still fail; report the concrete blocker instead of returning it to Ready.
5. Commit and push only the issue's changes.
6. Create a PR whose body links the issue, preferably with `Closes #<number>`. Ensure the PR is ready for review; if publication creates a draft, mark it ready before changing the card status.
7. Verify the remote PR URL, head branch, open state, non-draft state, and relevant check results.
8. Move the card to `Review` only after the implementation is pushed, the PR is ready for review, and the relevant local checks pass. Report any intentionally unrun checks.

Use the installed GitHub specialist workflows when applicable: the publish workflow for commit/push/PR creation, the review-comments workflow for actionable review threads, and the CI-fix workflow for failing GitHub Actions.

## Review and CI loop

For every follow-up that requires changes:

1. Re-read the card and PR state.
2. Move `Review` to `In Progress` before editing.
3. Address only the requested changes, run relevant checks, commit, and push to the same PR branch.
4. Keep `In Progress` while checks fail or work remains.
5. Move back to `Review` after verified fixes are pushed and the PR is again ready for review.

Do not toggle status for read-only questions, summaries, or review inspection that makes no changes.

## Completion and exceptional states

- Move to `Done` only after GitHub confirms the PR was merged. Do not equate approval, green CI, or a closed issue with a merged PR.
- If the PR is closed unmerged and the task remains active, move to `In Progress` and explain the next action.
- If the user abandons or cancels the task, ask which status they want; do not silently choose `Ready for agent` or `Backlog`.
- If Project mutation fails, continue no further state-dependent mutation, preserve the code/PR state, and report the exact failure and current known status.
- End every implementation turn with the issue URL, PR URL when present, current Project status, checks run, and any blocker.
