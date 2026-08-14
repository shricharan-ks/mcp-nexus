import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";
import { textSummary } from "https://jslib.k6.io/k6-summary/0.0.2/index.js";

// ---------------------------------------------------------------------------
// Custom metrics
// ---------------------------------------------------------------------------
const mcpToolCallDuration = new Trend("mcp_tool_call_duration", true);
const mcpToolCallErrors = new Rate("mcp_tool_call_errors");

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------
const GATEWAY_URL = __ENV.GATEWAY_URL || "http://localhost:8080";
const AUTH_TOKEN = __ENV.AUTH_TOKEN || "test-token";

export const options = {
  scenarios: {
    // Scenario 1 -- ramp VUs up to 500, then back down
    tool_calls: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "1m", target: 100 },
        { duration: "2m", target: 500 },
        { duration: "2m", target: 0 },
      ],
      gracefulRampDown: "10s",
    },

    // Scenario 2 -- constant 20 req/s for 3 minutes
    discovery: {
      executor: "constant-arrival-rate",
      rate: 20,
      timeUnit: "1s",
      duration: "3m",
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },

  thresholds: {
    http_req_duration: ["p(99)<500"],
    mcp_tool_call_errors: ["rate<0.01"],
    http_req_failed: ["rate<0.001"],
  },
};

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------
const headers = {
  "Content-Type": "application/json",
  Authorization: `Bearer ${AUTH_TOKEN}`,
};

// ---------------------------------------------------------------------------
// Scenario: tool_calls
// ---------------------------------------------------------------------------
export function tool_calls() {
  const payload = JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method: "tools/call",
    params: {
      name: "echo",
      arguments: { message: "load-test" },
    },
  });

  const res = http.post(`${GATEWAY_URL}/echo-server/mcp`, payload, {
    headers: Object.assign({}, headers, {
      "Mcp-Method": "tools/call",
      "Mcp-Name": "echo",
    }),
  });

  mcpToolCallDuration.add(res.timings.duration);
  mcpToolCallErrors.add(res.status !== 200);

  check(res, {
    "tool_call: status is 200": (r) => r.status === 200,
    "tool_call: response is valid JSON": (r) => {
      try {
        JSON.parse(r.body);
        return true;
      } catch (_) {
        return false;
      }
    },
  });

  sleep(0.1);
}

// ---------------------------------------------------------------------------
// Scenario: discovery
// ---------------------------------------------------------------------------
export function discovery() {
  const payload = JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method: "tools/list",
    params: {},
  });

  const res = http.post(`${GATEWAY_URL}/echo-server/mcp`, payload, {
    headers: Object.assign({}, headers, {
      "Mcp-Method": "tools/list",
    }),
  });

  check(res, {
    "discovery: status is 200": (r) => r.status === 200,
    "discovery: response contains tools array": (r) => {
      try {
        const body = JSON.parse(r.body);
        return (
          body.result !== undefined && Array.isArray(body.result.tools)
        );
      } catch (_) {
        return false;
      }
    },
  });

  sleep(0.1);
}

// ---------------------------------------------------------------------------
// Summary -- write JSON results alongside the script
// ---------------------------------------------------------------------------
export function handleSummary(data) {
  return {
    stdout: textSummary(data, { indent: "  ", enableColors: true }),
    "test/load/results.json": JSON.stringify(data, null, 2),
  };
}
