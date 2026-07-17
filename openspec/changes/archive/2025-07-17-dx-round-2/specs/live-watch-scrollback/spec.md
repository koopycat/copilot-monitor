## ADDED Requirements

### Requirement: live --watch preserves terminal scrollback

The `live --watch` command SHALL use cursor-up ANSI clearing
(`\x1b[<N>A\x1b[0J`) instead of full-screen clear (`\x1b[2J\x1b[H`) when
refreshing the display.

The command SHALL track the number of lines written in the previous render and
move the cursor up by exactly that count before each re-render. The command
SHALL NOT clear terminal scrollback history.

#### Scenario: First render

- **WHEN** `copilot-monitor live --watch` renders the live session for the first
  time
- **THEN** the display SHALL appear at the current cursor position without
  clearing the screen

#### Scenario: Subsequent refresh

- **WHEN** the live watch refreshes after 2 seconds
- **THEN** the previous N lines of live session output SHALL be overwritten in
  place
- **AND** terminal scrollback above those N lines SHALL be preserved

#### Scenario: Session state change between refreshes

- **WHEN** a new session starts between refreshes, changing the line count of
  the live display
- **THEN** the cursor-up offset SHALL use the previously rendered line count
- **AND** any extra lines from the old render beyond the new render SHALL be
  cleared
