# website-onboarding Specification

## Purpose

Define the minimal, accessible landing-page paths that take VS Code and pi users
from installation to their first captured request.

## Requirements

### Requirement: Landing page provides a minimal first-capture path

The public landing page SHALL present a getting-started section near the primary
installation controls that leads a developer from an installed binary to a first
captured request and the local dashboard. The section SHALL focus on the minimum
operational steps and defer advanced configuration to detailed documentation.

#### Scenario: Visitor starts onboarding

- **WHEN** a visitor has downloaded or installed Copilot Monitor and continues
  into the landing-page content
- **THEN** the visitor encounters the getting-started section before secondary
  product-detail sections
- **AND** the section identifies the first captured request and dashboard as the
  completion outcome

#### Scenario: Visitor needs local setup help

- **WHEN** a visitor cannot reach the dashboard or does not see a capture after
  completing the setup path
- **THEN** the landing page provides a `copilot-monitor doctor` command that
  checks the local monitor setup
- **AND** the guidance does not claim to inspect the visitor's editor settings

### Requirement: Onboarding provides separate VS Code and pi procedures

The getting-started section SHALL provide switchable procedures for VS Code with
GitHub Copilot and pi through an OpenRouter base URL override. Each procedure
SHALL include the commands or configuration needed to start Copilot Monitor,
route the selected client through it, generate a request, and open the
dashboard. Only the selected procedure SHALL be visually expanded when
JavaScript is available.

#### Scenario: Visitor selects VS Code

- **WHEN** the visitor activates the VS Code option
- **THEN** the page shows the VS Code Copilot proxy setting, reload action,
  monitor startup command, and dashboard destination
- **AND** pi-specific gateway setup is not visually expanded

#### Scenario: Visitor selects pi

- **WHEN** the visitor activates the pi option
- **THEN** the page shows the OpenRouter prerequisite, monitor startup command,
  pi base URL override, and dashboard destination
- **AND** VS Code-specific configuration is not visually expanded

### Requirement: Agent selector is accessible and resilient

The agent selector SHALL follow standard tab interaction semantics for pointer,
keyboard, and assistive-technology users. The setup instructions SHALL remain
available when JavaScript is disabled and SHALL remain usable without page-level
horizontal overflow on narrow viewports.

#### Scenario: Keyboard user changes procedure

- **WHEN** focus is within the agent tab list and the user presses Arrow Left,
  Arrow Right, Home, or End
- **THEN** focus and selected state move according to the standard tab pattern
- **AND** the corresponding procedure becomes the visually expanded panel

#### Scenario: JavaScript is unavailable

- **WHEN** the landing page loads without JavaScript
- **THEN** both procedures remain visible with clear headings
- **AND** all commands and links required for either setup remain available

#### Scenario: Visitor uses a narrow viewport

- **WHEN** the landing page is viewed at a narrow mobile width
- **THEN** the selector, step content, and command examples fit the content
  column
- **AND** long command examples scroll within their own containers rather than
  widening the page
