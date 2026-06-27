# RTK - Rust Token Killer

**Usage**: Token-optimized CLI proxy for shell commands.

## Rule

In Linux environment, always prefix shell commands with `rtk` to minimize token consumption.

Examples:

```bash
rtk git status
rtk cargo test
rtk ls src/
rtk grep "pattern" src/
rtk find "*.rs" .
rtk npm run build
rtk pytest -q
```

## Meta Commands

```bash
rtk gain            # Show token savings
rtk gain --history  # Command history with savings
rtk discover        # Find missed RTK opportunities
rtk proxy <cmd>     # Run raw (no filtering, for debugging)
```

## Why

RTK filters and compresses command output before it reaches the LLM context, saving 60-90% tokens on common operations. Always use `rtk <cmd>` instead of raw commands.
