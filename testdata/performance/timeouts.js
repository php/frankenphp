import http from "k6/http";

/**
 * Databases or external resources can sometimes become unavailable for short periods of time.
 * Make sure the server can recover quickly from periods of unavailability.
 * This simulation swaps between a hanging and a working server every 10 seconds.
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
  const tenSecondInterval = Math.floor(new Date().getSeconds() / 10);
  const shouldHang = tenSecondInterval % 2 === 0;

  // every 10 seconds requests lead to a max_execution-timeout
  if (shouldHang) {
    http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=50000`);
    return;
  }

  // every other 10 seconds the resource is back
  http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=5&work=30000&output=100`);
}
