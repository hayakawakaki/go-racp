# Contributing

Thanks for your interest in contributing. This guide covers how to propose changes and the quality bar every contribution is held to.

## Before you start

- Search existing issues and pull requests to avoid duplicates.
- For anything non-trivial, open an issue first so the approach can be discussed before you write code.
- Small fixes (typos, obvious bugs) can go straight to a pull request.

## Workflow

1. Fork the repository and create a branch off `master`.
2. Make your change in focused commits.
3. Make sure the quality requirements below all pass.
4. Open a pull request describing what changed and why.

## Quality requirements

Every pull request must meet all three of these before it can be merged.

### No code debt

- Solve the problem properly rather than working around it.
- No commented-out code, no dead code, no `TODO` left behind for someone else.
- No quick hacks that you expect to revisit later. If a shortcut is unavoidable, explain it in the pull request and open a follow-up issue.
- Keep changes scoped. Do not mix unrelated refactors into a feature or fix.

### Tests are required

- New features and bug fixes must come with tests that cover the change.
- A bug fix should include a test that fails before the fix and passes after it.
- Do not lower or skip existing tests to make a change pass.
- The full test suite must be green before you request review.

### No lint violations

- Code must pass the linter with zero issues.
- Fix the underlying problem rather than suppressing the warning. Reach for a suppression directive only as a last resort, and justify it in the pull request.
- Formatting must match the project standard. Run the formatter before committing.

## Pull requests

- Keep each pull request focused on a single concern.
- Write a clear description: what changed, why, and how it was tested.
- Link the issue it resolves.
- Be ready to revise based on review feedback.

## Reporting bugs and requesting features

Use the issue templates. Provide enough detail to reproduce a bug or to understand the problem a feature would solve.
