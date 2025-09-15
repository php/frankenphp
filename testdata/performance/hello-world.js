import http from "k6/http";

/**
 * 'Hello world' tests the raw server performance.
 */
export const options = {
  stages: [
    { duration: "5s", target: 100 },
    { duration: "20s", target: 400 },
    { duration: "5s", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.01"],
  },
};

/* global __ENV */
export default function () {
  http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php`);
}
