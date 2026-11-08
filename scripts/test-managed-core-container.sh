#!/usr/bin/env bash
# Isolated Linux TUN/desktop proxy acceptance environment. Native systemd
# lifecycle tests run separately through test-managed-core-linux.sh service.
set -euo pipefail
task_keep=false
while [[ "${1:-}" == --* ]]; do
  case "$1" in
    --keep) task_keep=true ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
  shift
done
task_core="$(realpath -- "${1:?usage: $0 [--keep] /path/to/verified/mihomo}")"
[[ -f "$task_core" && -x "$task_core" ]]
task_repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
task_temp="$(mktemp -d /tmp/mihosh-lab.XXXXXX)"
task_nonce="${task_temp##*.}"
task_anchor="mihosh-lab-${task_nonce}-anchor"
task_names=()
task_cleanup() {
  task_result=$?
  if [[ "$task_keep" == true ]]; then
    for task_name in "${task_names[@]}"; do echo "Retained environment: docker exec -it $task_name bash"; done
    echo "Remove when finished: docker rm -f $task_anchor"
  else
    for ((task_i=${#task_names[@]}-1; task_i>=0; task_i--)); do docker rm -f "${task_names[task_i]}" >/dev/null 2>&1 || true; done
  fi
  rm -rf -- "$task_temp"
  return "$task_result"
}
trap task_cleanup EXIT
cd -- "$task_repo"
CGO_ENABLED=0 GOMAXPROCS=2 GOMEMLIMIT=192MiB mise exec go -- go build -p 2 -o "$task_temp/mihosh" .
CGO_ENABLED=0 GOMAXPROCS=2 GOMEMLIMIT=192MiB mise exec go -- go test -p 2 -c -o "$task_temp/service.test" ./internal/app/service
# The build context contains only this Dockerfile.
docker build -t mihosh-native-linux-test:local scripts/native-linux
docker run --detach --name "$task_anchor" --network none --user 0 \
  --cap-add NET_ADMIN --cap-add SYS_ADMIN --device-cgroup-rule 'c 10:200 rwm' \
  --security-opt seccomp=unconfined --cgroupns private --memory 512m --cpus 2 --pids-limit 512 \
  --env HOME=/root --tmpfs /run mihosh-native-linux-test:local >/dev/null
task_names+=("$task_anchor")
task_target="$task_anchor"

docker cp "$task_temp/mihosh" "$task_target:/tmp/mihosh"
docker cp "$task_temp/service.test" "$task_target:/tmp/service.test"
docker cp "$task_core" "$task_target:/tmp/mihomo"
task_env=(-e MIHOSH_INTEGRATION_APP=/tmp/mihosh -e MIHOSH_INTEGRATION_BINARY=/tmp/mihomo -e HOME=/root)
docker exec "$task_target" sh -eu -c 'mkdir -p /dev/net /run/user/0; chmod 0700 /run/user/0; test -c /dev/net/tun || mknod /dev/net/tun c 10 200; chmod 0666 /dev/net/tun'
docker exec "${task_env[@]}" -e MIHOSH_NATIVE_TUN_TEST=1 "$task_target" \
  unshare -n sh -eu -c 'ip link set lo up; ip link add mihosh-test0 type dummy; ip addr add 192.0.2.1/30 dev mihosh-test0; ip link set mihosh-test0 up; ip route add default dev mihosh-test0; exec /tmp/service.test -test.run="^TestNativeManagedTUN$" -test.v -test.count=1'
docker exec "${task_env[@]}" -e XDG_RUNTIME_DIR=/run/user/0 -e MIHOSH_NATIVE_PROXY_TEST=1 -e MIHOSH_ALLOW_NETWORK_CHANGES=1 "$task_target" \
  dbus-run-session -- /tmp/service.test -test.run='^TestNativeManagedSystemProxy$' -test.v -test.count=1
