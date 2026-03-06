import http from "k6/http";

/**
 * It is not uncommon for external services to hang for a long time.
 * Make sure the server is resilient in such cases and doesn't hang as well.
 */
export const options = {
  stages: [
    { duration: "20s", target: 100 },
    { duration: "20s", target: 500 },
    { duration: "20s", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.01"],
  },
};

/* global __ENV */
export default function () {
  // 2% chance for a request that hangs for 15s
  if (Math.random() < 0.02) {
    http.get(
      `${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=15000&work=10000&output=100`,
    );
    return;
  }

  // a regular request
  http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=5&work=10000&output=100`);
}
