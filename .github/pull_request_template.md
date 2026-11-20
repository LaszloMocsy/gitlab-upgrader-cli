## What and why

<!-- What does this change, and why? Link the issue it closes. -->

## How it was tested

<!-- Unit tests, fixtures, a real instance (which install type and distro)? -->

## Checklist

- [ ] Tests cover the new or changed behaviour
- [ ] No `os/exec` outside `internal/runner`; Linux-only code is behind build tags
- [ ] No setup-specific paths or assumptions (detected or configurable instead)
- [ ] Docs updated if behaviour or flags changed
