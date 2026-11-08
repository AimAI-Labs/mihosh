#!/usr/bin/env bash
# Native checks are opt-in. Service mode installs only a uniquely named temporary
# user service. TUN mode runs inside a separate user/network namespace.
set -euo pipefail
task_mode="${1:-isolated}"
task_core="$(realpath -- "${2:?usage: $0 isolated|service|tun|proxy /path/to/verified/mihomo [/path/to/mihosh]}")"
task_repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
task_app="$(realpath -- "${3:-$task_repo/mihosh}")"
[[ -f "$task_core" && -x "$task_core" && -f "$task_app" && -x "$task_app" ]]
task_test="$(mktemp /tmp/mihosh-native-test.XXXXXX)"
trap 'rm -f -- "$task_test"' EXIT
cd -- "$task_repo"
GOMAXPROCS=2 GOMEMLIMIT=192MiB mise exec go -- go test -p 2 -c -o "$task_test" ./internal/app/service
export MIHOSH_INTEGRATION_BINARY="$task_core" MIHOSH_INTEGRATION_APP="$task_app"
unset MIHOSH_NATIVE_SERVICE_TEST MIHOSH_NATIVE_TUN_TEST MIHOSH_NATIVE_PROXY_TEST
case "$task_mode" in
  isolated)
    unshare -Urn sh -eu -c 'ip link set lo up; exec "$1" -test.run="^TestCoreReal(Binary|BackgroundAndNetworkGuard)$" -test.v -test.count=1' sh "$task_test"
    ;;
  service)
    export MIHOSH_NATIVE_SERVICE_TEST=1
    "$task_test" -test.run='^TestNativeManagedService$' -test.v -test.count=1
    ;;
  tun)
    [[ -c /dev/net/tun ]] || { echo '/dev/net/tun is required on the test host' >&2; exit 1; }
    export MIHOSH_NATIVE_TUN_TEST=1
    unshare -Urn sh -eu -c 'ip link set lo up; ip link add mihosh-test0 type dummy; ip address add 192.0.2.1/30 dev mihosh-test0; ip link set mihosh-test0 up; ip route add default dev mihosh-test0; exec "$1" -test.run="^TestNativeManagedTUN$" -test.v -test.count=1' sh "$task_test"
    ;;
  proxy)
    [[ "${MIHOSH_ALLOW_NETWORK_CHANGES:-}" == 1 ]] || { echo 'Native desktop proxy test requires MIHOSH_ALLOW_NETWORK_CHANGES=1' >&2; exit 1; }
    export MIHOSH_NATIVE_PROXY_TEST=1
    "$task_test" -test.run='^TestNativeManagedSystemProxy$' -test.v -test.count=1
    ;;
  *) echo "Unknown test mode: $task_mode" >&2; exit 1 ;;
esac
