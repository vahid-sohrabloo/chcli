#!/usr/bin/env bash
# Run the chtop integration tests against each supported ClickHouse version.
#
# Starts a clickhouse-server Docker container per version, waits for it to be
# ready, exports the CHCLI_TEST_* env vars expected by the tests, runs
# `go test ./internal/chtop/...` (and optionally the full suite), then tears
# the container down. Aggregates pass/fail across versions and exits non-zero
# if any version failed.
#
# Usage:
#   scripts/test-versions.sh                # all versions
#   scripts/test-versions.sh 25.3 latest    # a subset
#   CHCLI_TEST_ALL=1 scripts/test-versions.sh  # run ./... not just chtop
set -eo pipefail

# Matches the matrix in .github/workflows/ci.yml.
ALL_VERSIONS=(24.8 25.3 25.8 26.3 latest)

VERSIONS=("${@:-${ALL_VERSIONS[@]}}")
if [[ $# -gt 0 ]]; then
	VERSIONS=("$@")
fi

# Pick a high port to dodge collisions with a dev clickhouse-server.
PORT="${CHCLI_TEST_PORT:-29000}"
IMAGE_NAME="clickhouse/clickhouse-server"
CONTAINER_NAME_PREFIX="chcli-it"

# Default to running only the chtop integration tests. Set CHCLI_TEST_ALL=1
# to run the whole suite.
if [[ -n "${CHCLI_TEST_ALL:-}" ]]; then
	TEST_TARGET="./..."
else
	TEST_TARGET="./internal/chtop/..."
fi

cleanup() {
	if [[ -n "${CONTAINER:-}" ]]; then
		docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

PASSED=()
FAILED=()

for version in "${VERSIONS[@]}"; do
	printf '\n\033[1m==> ClickHouse %s\033[0m\n' "$version"
	CONTAINER="${CONTAINER_NAME_PREFIX}-${version//[^a-zA-Z0-9]/_}"

	# In case a previous run left one behind.
	docker rm -f "$CONTAINER" >/dev/null 2>&1 || true

	echo "    starting container: $CONTAINER ($IMAGE_NAME:$version on :$PORT)"
	docker run -d --rm \
		--name "$CONTAINER" \
		-p "${PORT}:9000" \
		-e CLICKHOUSE_SKIP_USER_SETUP=1 \
		"$IMAGE_NAME:$version" >/dev/null

	# Wait for the server to accept queries. Cap at ~60s.
	echo -n "    waiting for clickhouse-server "
	ready=0
	for i in {1..60}; do
		if docker exec "$CONTAINER" clickhouse-client --query 'SELECT 1' >/dev/null 2>&1; then
			ready=1
			break
		fi
		echo -n "."
		sleep 1
	done
	echo
	if [[ "$ready" -ne 1 ]]; then
		echo "    ! server did not become ready in 60s; container logs:"
		docker logs --tail 30 "$CONTAINER" | sed 's/^/      /'
		FAILED+=("$version (startup timeout)")
		docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
		CONTAINER=
		continue
	fi

	# Run the tests.
	set +e
	CHCLI_TEST_HOST=localhost \
	CHCLI_TEST_CONNSTR="clickhouse://default@localhost:${PORT}/default" \
		go test -race -count=1 -timeout 120s "$TEST_TARGET"
	status=$?
	set -e

	# Tear down before moving on.
	docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
	CONTAINER=

	if [[ "$status" -eq 0 ]]; then
		PASSED+=("$version")
		printf '    \033[32mPASS\033[0m %s\n' "$version"
	else
		FAILED+=("$version")
		printf '    \033[31mFAIL\033[0m %s\n' "$version"
	fi
done

printf '\n\033[1m=== Summary ===\033[0m\n'
for v in "${PASSED[@]}"; do printf '  \033[32m✓\033[0m %s\n' "$v"; done
for v in "${FAILED[@]}"; do printf '  \033[31m✗\033[0m %s\n' "$v"; done

if [[ "${#FAILED[@]}" -gt 0 ]]; then
	exit 1
fi
