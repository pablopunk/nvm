#!/usr/bin/env bash
# Drives the L4 rig container (scripts/rig/Dockerfile). Run from anywhere; paths are resolved here.
#
#   rig.sh build                  build the image (Go version from go.mod)
#   rig.sh run [go test args]     run the journeys in a fresh container (default: -run Journey -v)
#   rig.sh up                     start a long-lived container with the desktop, for shot and manual runs
#   rig.sh shot <out.png>         screenshot the display of the running container
#   rig.sh logs                   container log and the desktop service logs of the last run
#   rig.sh down                   remove the long-lived container
#
# Environment:
#   RIG_IMAGE       image tag (default nvm-rig:local)
#   RIG_CONTAINER   long-lived container name (default nvm-rig)
#   RIG_ARTIFACTS   host directory for screenshots, test logs and service logs
#                   (default for run: a new directory per run under $TMPDIR; otherwise $TMPDIR/nvm-rig-artifacts)
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
native_dir="$(cd "$script_dir/../.." && pwd)"
image="${RIG_IMAGE:-nvm-rig:local}"
container="${RIG_CONTAINER:-nvm-rig}"
artifacts="${RIG_ARTIFACTS:-${TMPDIR:-/tmp}/nvm-rig-artifacts}"

go_version() {
  awk '$1 == "go" { print $2; exit }' "$native_dir/go.mod"
}

docker_common_flags() {
  printf '%s\n' --init --shm-size=512m \
    -v "$native_dir:/work" \
    -v nvm-rig-gomod:/root/go/pkg/mod \
    -v nvm-rig-gobuild:/root/.cache/go-build \
    -v "$artifacts:/artifacts" \
    -e RIG_ARTIFACTS=/artifacts/services \
    -e NVM_RIG_ARTIFACTS=/artifacts/tests \
    -e NVM_RIG_BIN=/tmp/nvm-rig
}

build_image() {
  docker build -f "$script_dir/Dockerfile" --build-arg "GO_VERSION=$(go_version)" -t "$image" "$native_dir"
}

# run_journeys executes the app build and the rig tests inside a fresh container, then exits with
# the test's status. Artifacts land in $artifacts even when the tests fail.
run_journeys() {
  if [ -z "${RIG_ARTIFACTS:-}" ]; then
    artifacts="$(mktemp -d "${TMPDIR:-/tmp}/nvm-rig-artifacts.XXXXXX")"
  fi
  echo "rig: artifacts in $artifacts"
  mkdir -p "$artifacts"
  local tests=("$@")
  if [ "${#tests[@]}" -eq 0 ]; then
    tests=(-run Journey -v)
  fi
  local flags=()
  while IFS= read -r flag; do flags+=("$flag"); done < <(docker_common_flags)
  docker run --rm "${flags[@]}" "$image" bash scripts/rig/run-journeys.sh "${tests[@]}"
}

up() {
  mkdir -p "$artifacts"
  local flags=()
  while IFS= read -r flag; do flags+=("$flag"); done < <(docker_common_flags)
  docker run -d --name "$container" "${flags[@]}" "$image" sleep infinity >/dev/null
  echo "rig: container $container up; desktop on \$RIG_DISPLAY inside the container (default :99)"
}

shot() {
  local out="${1:?usage: rig.sh shot <out.png>}"
  # -w /tmp: the container's working directory is a bind mount that may have been replaced on the host.
  docker exec -w /tmp "$container" bash -c 'DISPLAY="${RIG_DISPLAY:-:99}" import -window root /tmp/rig-shot.png'
  docker cp "$container:/tmp/rig-shot.png" "$out"
  echo "rig: screenshot $out"
}

logs() {
  docker logs "$container" 2>&1 | tail -n 200 || true
  echo "rig: service logs in $artifacts/services (if the container was started with it)"
}

down() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}

main() {
  local command="${1:-}"
  shift || true
  case "$command" in
    build) build_image ;;
    run) run_journeys "$@" ;;
    up) up ;;
    shot) shot "$@" ;;
    logs) logs ;;
    down) down ;;
    *)
      sed -n '2,13p' "${BASH_SOURCE[0]}" >&2
      return 2
      ;;
  esac
}

main "$@"
