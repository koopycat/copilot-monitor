## REMOVED Requirements

### Requirement: Browser-only routing in combined mode

**Reason**: Combined proxy+dashboard mode (`run --dashboard`) is removed. The
dashboard is always served on its own port via `copilot-monitor serve`. No
header sniffing needed.

**Migration**: Run `copilot-monitor serve` alongside `copilot-monitor run`
instead of using `--dashboard`.
