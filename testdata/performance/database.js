import http from "k6/http";

/**
 * Modern databases tend to have latencies in the single-digit milliseconds.
 * We'll simulate 1-10ms latencies and 1-2 queries per request.
 */
export const options = {
  stages: [
    { duration: "20s", target: 100 },
    { duration: "30s", target: 200 },
    { duration: "10s", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.01"],
  },
};

/* global __ENV */
export default function () {
  // 1-10ms latency
  const latency = Math.floor(Math.random() * 10) + 1;
  // 1-2 iterations per request
  const iterations = Math.floor(Math.random() * 2) + 1;
  // 1-30000 work units per iteration
  const work = Math.ceil(Math.random() * 30000);
  // 1-40 output units
  const output = Math.ceil(Math.random() * 40);

  http.get(
    http.url`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=${latency}&work=${work}&output=${output}&iterations=${iterations}`,
  );
}
