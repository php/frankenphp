#!/bin/bash

# install the dev.Dockerfile, build the app and run k6 tests

docker build -t frankenphp-dev -f dev.Dockerfile .

export "CADDY_HOSTNAME=http://host.docker.internal"

select filename in ./testdata/performance/*.js; do
	read -r -p "How many worker threads (40)? " workerThreads
	read -r -p "How many max threads (80)? " maxThreads

	workerThreads=${workerThreads:-40}
	maxThreads=${maxThreads:-80}

	numThreads=$((workerThreads + 1))

	docker run --cap-add=SYS_PTRACE --security-opt seccomp=unconfined \
		-p 8125:80 \
		-v "$PWD:/go/src/app" \
		--rm \
		--name load-test-container \
		-e "MAX_THREADS=$maxThreads" \
		-e "WORKER_THREADS=$workerThreads" \
		-e "NUM_THREADS=$numThreads" \
		-itd \
		frankenphp-dev \
		sh /go/src/app/testdata/performance/start-server.sh

	docker exec -d load-test-container sh /go/src/app/testdata/performance/flamegraph.sh

	sleep 10

	docker run --entrypoint "" -it --rm -v .:/app -w /app \
		--add-host "host.docker.internal:host-gateway" \
		grafana/k6:latest \
		k6 run -e "CADDY_HOSTNAME=$CADDY_HOSTNAME:8125" "./$filename"

	docker exec load-test-container curl "http://localhost:2019/frankenphp/threads"

	docker stop load-test-container
done
