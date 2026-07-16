---
name: use-platforma
description: Apply conventions when implementing, reviewing, or refactoring Go code that uses the Platforma framework.
---

# Use Platforma

## Logging

- Use `github.com/platforma-dev/platforma/log` for diagnostic logging.
- Prefer context-aware methods when a `context.Context` is available.
- Log errors as structured attributes: `"error", err`.
- Do not use `fmt.Fprintln(os.Stderr, err)`, ad hoc printing, or the standard `log` package for diagnostics.
- Use `fmt` only for intentional CLI output, prompts, or protocol payloads.

## HTTP

- Keep routes, handlers, transport DTOs, and error mapping in their owning domain package.
- Use the root HTTP package only to assemble and mount domain modules.
- Keep handler tests in the same domain test package.

## Testing

- Test only behavior observable through the package's exported API.
- Put tests in an external `_test` package, such as `package app_test`.
- Place test functions at the top of test files and helper functions and types below them.
