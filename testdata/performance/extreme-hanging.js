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

/* global __VU */
/* global __ENV */
export default function () {
    if (__VU % 50 === 0) {
        // 50 % of VUs cause extreme hanging
        http.get(`${__ENV.CADDY_HOSTNAME}/slow-path?sleep=60000&work=10000&output=100`)
    } else {
        // The other VUs do very fast requests
        http.get(`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=3&work=1000`)
    }
}
