#!/usr/bin/env bash
# Installs (or upgrades) temper-exporter as a systemd service on a Debian /
# Debian-derivative Linux box, from the latest GitHub release.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/BAPostma/temper-exporter/main/install.sh | sudo bash
set -euo pipefail

REPO="BAPostma/temper-exporter"
BIN_PATH="/usr/local/bin/temper_exporter"
CONFIG_DIR="/etc/temper-exporter"
CONFIG_PATH="$CONFIG_DIR/config.json"
SERVICE_PATH="/etc/systemd/system/temper-exporter.service"

if [ "$(id -u)" -ne 0 ]; then
  echo "error: this script must be run as root, e.g.:" >&2
  echo "  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | sudo bash" >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
  echo "error: systemctl not found; this script requires a systemd-based Linux system." >&2
  exit 1
fi

case "$(uname -m)" in
  aarch64) ARCH="arm64" ;;
  armv6l|armv7l|arm) ARCH="armv6" ;;
  *)
    echo "error: unsupported architecture '$(uname -m)'. Supported: arm64, armv6 (e.g. Raspberry Pi)." >&2
    exit 1
    ;;
esac

if ! command -v jq >/dev/null 2>&1; then
  echo "jq not found, installing..."
  apt-get update -qq
  apt-get install -y jq
fi

echo "Fetching latest release metadata for ${REPO}..."
release_json=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")

asset_json=$(echo "$release_json" | jq -r --arg arch "$ARCH" \
  '[.assets[] | select(.name | contains("linux-" + $arch))][0]')

if [ "$asset_json" = "null" ] || [ -z "$asset_json" ]; then
  echo "error: no release asset found for arch '${ARCH}' in the latest release of ${REPO}." >&2
  exit 1
fi

tag_name=$(echo "$release_json" | jq -r '.tag_name')
asset_name=$(echo "$asset_json" | jq -r '.name')
download_url=$(echo "$asset_json" | jq -r '.browser_download_url')
digest=$(echo "$asset_json" | jq -r '.digest // empty')

echo "Installing ${asset_name} (release ${tag_name})"

tmp_file=$(mktemp)
trap 'rm -f "$tmp_file"' EXIT

curl -fsSL -o "$tmp_file" "$download_url"

if [ -n "$digest" ]; then
  expected_sha="${digest#sha256:}"
  actual_sha=$(sha256sum "$tmp_file" | awk '{print $1}')
  if [ "$expected_sha" != "$actual_sha" ]; then
    echo "error: checksum mismatch for ${asset_name}" >&2
    echo "  expected: ${expected_sha}" >&2
    echo "  actual:   ${actual_sha}" >&2
    exit 1
  fi
  echo "Checksum verified (sha256)."
else
  echo "warning: no digest available from GitHub for this asset; skipping checksum verification." >&2
fi

install -m 0755 "$tmp_file" "$BIN_PATH"

mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_PATH" ]; then
  if curl -fsSL -o "$CONFIG_PATH" "https://raw.githubusercontent.com/${REPO}/main/config.json"; then
    echo "Wrote default config to ${CONFIG_PATH}"
  else
    echo "warning: failed to fetch default config.json; temper-exporter will use its built-in defaults." >&2
    rm -f "$CONFIG_PATH"
  fi
else
  echo "Existing config at ${CONFIG_PATH} left untouched."
fi

cat > "$SERVICE_PATH" <<EOF
[Unit]
Description=Temper USB exporter for Prometheus
After=network.target

[Service]
Type=simple
ExecStart=${BIN_PATH} -config ${CONFIG_PATH}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable temper-exporter.service
systemctl restart temper-exporter.service

echo
echo "temper-exporter ${tag_name} installed and running."
echo "  status:  systemctl status temper-exporter"
echo "  logs:    journalctl -u temper-exporter -f"
echo "  config:  edit ${CONFIG_PATH}, then 'systemctl restart temper-exporter'"
