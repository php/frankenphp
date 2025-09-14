import http from 'k6/http'

/**
 * It is not uncommon for external services to hang for a long time.
 * Make sure the server is resilient in such cases and doesn't hang as well.
 */
export const options = {
  stages: [
    { duration: '20s', target: 100 },
    { duration: '20s', target: 500 },
    { duration: '20s', target: 0 }
  ],
  thresholds: {
    http_req_failed: ['rate<0.5']
  }
}

/* global __ENV */
export default function () {
  // 2% chance for a request that hangs for 15s
  if (Math.random() < 0.02) {
    http.get(`${__ENV.CADDY_HOSTNAME}/slow-path?sleep=15000&work=10000&output=100`, {
        timeout: 500, // do not wait and continue with the next request
        throw: false,
    })
    return
  }

  // a regular request
  http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=5&work=10000&output=100`)
}
