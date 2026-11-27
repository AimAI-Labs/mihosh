#!/usr/bin/env bash
set -euo pipefail

installer="$(cd "$(dirname "$0")/.." && pwd)/install.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

mkdir "$fixture"/{bin,download,legacy,tmp}
export TMPDIR="$fixture/tmp"
export MIHOSH_FIXTURE="$fixture"
# Change only the install destination in the test copy.
sed "s|^INSTALL_DIR=.*|INSTALL_DIR=\"$fixture/install/bin\"|" "$installer" >"$fixture/install.sh"

cat >"$fixture/bin/uname" <<'MOCK'
#!/bin/bash
case "$1" in
    -s) echo Darwin ;;
    -m) echo arm64 ;;
esac
MOCK
cat >"$fixture/bin/curl" <<'MOCK'
#!/bin/bash
out=""
while [ "$#" -gt 1 ]; do
    if [ "$1" = "-o" ]; then
        out="$2"
        shift
    fi
    shift
done
case "$1" in
    *releases/latest)
        case "$MIHOSH_INSTALL_CASE" in
            rate_limit) exit 22 ;;
            missing_tag) echo '{"message":"missing tag"}' ;;
            *) echo '{"tag_name":"v2.0.1"}' ;;
        esac
        ;;
    */v2.0.1/mihosh-darwin-arm64.tar.gz)
        case "$MIHOSH_INSTALL_CASE" in
            corrupt) echo invalid >"$out" ;;
            *) cp "$MIHOSH_FIXTURE/download/new.tar.gz" "$out" ;;
        esac
        ;;
    *) exit 22 ;;
esac
MOCK
cat >"$fixture/bin/sudo" <<'MOCK'
#!/bin/bash
exec "$@"
MOCK
chmod +x "$fixture/bin/"*
printf '#!/bin/bash\necho old\n' >"$fixture/legacy/mihosh"
printf '#!/bin/bash\necho new\n' >"$fixture/download/mihosh"
tar -czf "$fixture/mihosh-darwin-arm64.tar.gz" -C "$fixture/legacy" mihosh
tar -czf "$fixture/download/new.tar.gz" -C "$fixture/download" mihosh
cd "$fixture"
export PATH="$fixture/bin:/usr/bin:/bin"

for failure in rate_limit missing_tag corrupt; do
    if MIHOSH_INSTALL_CASE="$failure" bash install.sh --install >"$fixture/output" 2>&1; then
        echo "Installer unexpectedly succeeded for $failure" >&2
        exit 1
    fi
    if [ "$failure" != corrupt ]; then
        grep -q 'Failed to fetch latest version info.' "$fixture/output"
    fi
    test ! -e "$fixture/install/bin/mihosh"
    test -z "$(find "$TMPDIR" -mindepth 1 -print)"
done

MIHOSH_INSTALL_CASE=success bash install.sh --install
test "$("$fixture/install/bin/mihosh")" = new
test -z "$(find "$TMPDIR" -mindepth 1 -print)"
echo "Installer handles API failures, ignores stale archives and installs into a missing bin directory."
