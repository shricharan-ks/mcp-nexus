# MCP Gateway Load Tests

k6 load tests for MCP Gateway Phase 7.

## Prerequisites

- [k6](https://k6.io/docs/getting-started/installation/) installed (`brew install k6` on macOS).

## Running

Default run against `http://localhost:8080`:

```bash
k6 run test/load/k6-script.js
```

Custom gateway URL:

```bash
k6 run -e GATEWAY_URL=http://gateway.example.com test/load/k6-script.js
```

Custom auth token:

```bash
k6 run -e AUTH_TOKEN=my-token test/load/k6-script.js
```

Smaller-scale smoke run (10 VUs for 1 minute, overrides scenarios):

```bash
k6 run --vus 10 --duration 1m test/load/k6-script.js
```

## Thresholds

| Metric | Condition | Purpose |
|---|---|---|
| `http_req_duration` | p(99) < 500 ms | 99th-percentile latency must stay under 500 ms |
| `mcp_tool_call_errors` | rate < 1 % | Less than 1 % of tool calls may return a non-200 status |
| `http_req_failed` | rate < 0.1 % | Overall HTTP failure rate must stay below 0.1 % |

If any threshold is exceeded k6 exits with a non-zero code, making it suitable for CI gates.

## Results

After a run, a JSON summary is written to `test/load/results.json`.
