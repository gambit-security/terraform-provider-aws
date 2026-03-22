# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## GitHub Actions Security

All GitHub Actions in workflows **must be pinned to full commit SHAs** (not tags or branches) for supply chain security. The only exception is actions from `gambit-security/shared-workflows`, which may use branch/tag references.

```yaml
# WRONG - using tag
- uses: actions/checkout@v4

# CORRECT - pinned to SHA
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2

# CORRECT - shared-workflows exception (no SHA needed)
- uses: gambit-security/shared-workflows/.github/workflows/build.yml@main
```

Use `pinact` to pin actions: `pinact run --exclude "gambit-security/shared-workflows"`
