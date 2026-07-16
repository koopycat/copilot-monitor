<!-- markdownlint-disable MD041 -->

## Purpose

Detect when requests arrive through an external Headroom compression proxy and
flag them with a `headroom_proxied` indicator. No inline compression
transformation is performed by Copilot Monitor.

## Requirements

### Requirement: Headroom proxy detection

The system SHALL detect requests arriving from a Headroom compression proxy by
checking the RemoteAddr of the incoming connection against a configured
headroom-proxy address. When detected, the request SHALL be flagged with
`headroom_proxied = true` in the stored record.

#### Scenario: Request from Headroom detected

- **WHEN** a request arrives from `127.0.0.1:8787` and `--headroom-proxy-addr`
  is set to `127.0.0.1:8787`
- **THEN** the request is flagged `headroom_proxied = true` in the database

#### Scenario: Request from other source

- **WHEN** a request arrives from `127.0.0.1:54321` and `--headroom-proxy-addr`
  is set to `127.0.0.1:8787`
- **THEN** the request is flagged `headroom_proxied = false`

#### Scenario: Default headroom-proxy-addr

- **WHEN** no `--headroom-proxy-addr` is provided
- **THEN** the default of `127.0.0.1:8787` is used

---

### Requirement: Headroom-proxy-addr configuration

The `--headroom-proxy-addr` flag SHALL be available on the `run` command. It
SHALL accept a host:port value. It is independent of the `--upstream` flag.

#### Scenario: Custom headroom address

- **WHEN** `--headroom-proxy-addr 127.0.0.1:9797` is set
- **THEN** RemoteAddr matching uses `127.0.0.1:9797`

---

### Requirement: No inline compression

Copilot Monitor SHALL NOT perform any compression transformation on request
bodies. Compression, when desired, is handled by an external Headroom proxy
running as a separate process in front of Copilot Monitor.

#### Scenario: No compression callout

- **WHEN** a request arrives
- **THEN** the proxy does not call a compression endpoint, does not modify
  request bodies, and does not attempt to decompress responses

---

### Requirement: headroom_proxied in export

The `headroom_proxied` flag SHALL be included in CSV export output as a column.

#### Scenario: Export includes headroom_proxied

- **WHEN** `copilot-monitor export` is run
- **THEN** the CSV includes a `headroom_proxied` column with `true` or `false`
  values
