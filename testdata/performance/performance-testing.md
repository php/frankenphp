# Running Load tests

To run load tests with k6 you need to have Docker and Bash installed.
Go the root of this repository and run:

```sh
bash testdata/performance/perf-test.sh
```

This will build the `frankenphp-dev` Docker image and run it under the name 'load-test-container'
in the background. Additionally, it will run the `grafana/k6` container, and you'll be able to choose
the load test you want to run. A `flamegraph.svg` will be created in the `testdata/performance` directory.
