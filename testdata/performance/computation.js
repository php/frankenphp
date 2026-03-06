import http from "k6/http";

/**
 * Simulate an application that does very little IO, but a lot of computation
 */
export const options = {
  stages: [
    { duration: "20s", target: 80 },
    { duration: "20s", target: 150 },
    { duration: "5s", target: 0 },
  ],
  thresholds: {
    http_req_failed: ["rate<0.01"],
  },
};

/* global __ENV */
export default function () {
  // do 1-1,000,000 work units
  const work = Math.ceil(Math.random() * 1_000_000);
  // output 1-500 units
  const output = Math.ceil(Math.random() * 500);
  // simulate 0-2ms latency
  const latency = Math.floor(Math.random() * 3);

  http.get(
    http.url`${__ENV.CADDY_HOSTNAME}/sleep.php?sleep=${latency}&work=${work}&output=${output}`,
  );
}
