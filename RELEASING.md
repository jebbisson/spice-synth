# Releasing

These repositories use an automatic tag workflow on pushes to `main`.

## Default Behavior

- if `HEAD` already has a semver tag, no new tag is created
- otherwise the workflow creates the next semantic version tag automatically
- default bump is `patch`

## Commit Message Overrides

Include one of these markers anywhere in the commit message:

- `[minor]` to bump minor and reset patch
- `[major]` to bump major and reset minor/patch

If neither marker is present, the workflow bumps the patch version.

## Examples

- `fix loader bug` -> patch bump
- `add shared loader [minor]` -> minor bump
- `break backend ABI [major]` -> major bump

## Notes

- tags are created from pushes to `main`
- this is meant to reduce manual tagging overhead, not replace release review
- if you need stricter release controls later, this can be replaced with a manual dispatch or release-please style workflow
