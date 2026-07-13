<!-- markdownlint-disable MD041 -->

## ADDED Requirements

### Requirement: Quick-reference section in AGENTS.md

AGENTS.md SHALL contain a "Quick Reference" section at the top that lists the
most common agent workflows as concrete `just` commands. The section SHALL cover
build, test, format, and check operations in a scannable format.

#### Scenario: Agent reads AGENTS.md for workflow guidance

- **WHEN** an AI agent reads AGENTS.md
- **THEN** the Quick Reference section is visible near the top of the file
- **AND** it lists concrete commands for building, testing, formatting, and
  checking
- **AND** each command fits on a single line

### Requirement: Predictable justfile structure

The justfile SHALL be organized into logical sections with descriptive comment
headers. Sections SHALL cover at minimum: Build, Test, Check, Format, Dev.

#### Scenario: Agent discovers available targets

- **WHEN** an agent reads the justfile section headers
- **THEN** each section group is clearly delimited and labeled
- **AND** the agent can identify the right section for their task without
  reading the entire file

### Requirement: Agent setup prerequisites documented

AGENTS.md SHALL list the prerequisites for working in the repo, including
direnv, devenv, and pre-commit installation. The setup instructions SHALL be
reproducible in a single `just setup` invocation.

#### Scenario: Agent sets up the repo from scratch

- **WHEN** an agent follows the AGENTS.md setup steps on a clean machine
- **THEN** all required tooling is installed and functional
- **AND** the development environment is active
