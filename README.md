# tidegate

Tidal sluice **gate interlock and head-difference controller** for estuary lock chambers. Tidegate ingests signed water-level and gate-position sensor reports, smooths probe readings, evaluates interlock matrices and head-diff limits, issues short-lived permit tickets, and dispatches commands to a PLC. An embedded operator console shows chamber state and recent denials.

## Requirements

- Go 1.22+
- CGO disabled (`CGO_ENABLED=0`) — stdlib only

## Build

```bat
set GOTOOLCHAIN=local
set CGO_ENABLED=0
go build ./...
```

Build the server binary:

```bat
go build -o tidegate.exe ./cmd/tidegate
```

## Run

```bat
set TIDEGATE_LISTEN=:8080
set TIDEGATE_HMAC_SECRET=dev-hmac-secret-change-me
go run ./cmd/tidegate
```

Open the operator console at `http://localhost:8080/`.

### Configuration (environment)

| Variable | Default | Description |
|----------|---------|-------------|
| `TIDEGATE_LISTEN` | `:8080` | HTTP listen address |
| `TIDEGATE_HMAC_SECRET` | dev default | Sensor HMAC secret |
| `TIDEGATE_HEAD_DIFF_LIMIT_CM` | `30` | Max allowed head difference (cm) |
| `TIDEGATE_SMOOTH_N` | `5` | Median smoothing window |
| `TIDEGATE_SKEW_SEC` | `120` | Allowed timestamp skew |
| `TIDEGATE_PLC_ENDPOINT` | local URL | PLC permit dispatch endpoint |
| `TIDEGATE_JOURNAL_PATH` | `tidegate.journal` | Append-only journal file |
| `TIDEGATE_BYPASS_ENABLED` | `false` | Allow interlock bypass token |

## Test

```bat
set GOTOOLCHAIN=local
go test ./... -count=1
```

## API overview

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/sensors/level` | Signed upstream/downstream level ingest |
| `POST` | `/v1/sensors/gate` | Signed gate encoder ingest |
| `POST` | `/v1/ops/request-open` | Request open/close permit |
| `GET` | `/v1/status` | Chamber/gate snapshot |
| `GET` | `/` | Operator console (embedded static UI) |

Sensor requests require headers `X-Tide-Key` and `X-Tide-Time` (HMAC-SHA256 over timestamp + body).

## Project layout

```
cmd/tidegate/          Entry point
internal/config/       Thresholds and secrets
internal/model/        Domain types
internal/level/        Median smoothing and head diff
internal/gatefsm/      Gate state machine
internal/interlock/    Mutual exclusion matrix
internal/permit/       Permit ticket issuance
internal/ingest/       HMAC sensor HTTP handlers
internal/dispatch/     PLC dispatch + circuit breaker
internal/journal/      Operational journal
internal/app/          Wiring and HTTP routes
internal/web/static/   Embedded operator UI
web/                   Source UI assets
```

## License

Proprietary — internal use.
