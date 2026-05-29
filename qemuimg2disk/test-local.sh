#!/usr/bin/env bash
# Local repro for the qemuimg2disk action.
#
# The action runs `qemu-img convert -O host_device "$DEST_DISK"` and
# host_device only accepts real block/char devices. To test locally we
# back a temp raw file with a loop device and pass that through.
#
# Requires root (losetup + /dev/loopN access).
#
# Usage:
#   sudo ./test-local.sh
#   sudo IMG_URL=... ./test-local.sh
#   sudo IMAGE=qemuimg2disk:local ./test-local.sh

set -o errexit
set -o nounset
set -o pipefail

if [[ "${EUID}" -ne 0 ]]; then
    echo "must be run as root (need losetup + block device access)" >&2
    exit 1
fi

IMG_URL="${IMG_URL:-https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img}"
FORMAT="${FORMAT:-host_device}"
SIZE="${SIZE:-10G}"
IMAGE="${IMAGE:-quay.io/tinkerbell/actions/qemuimg2disk:latest}"

workdir="$(mktemp -d -t qemuimg2disk-XXXXXX)"
disk="${workdir}/disk.raw"
loopdev=""

cleanup() {
    if [[ -n "${loopdev}" ]]; then
        losetup -d "${loopdev}" 2>/dev/null || true
    fi
    rm -rf "${workdir}"
}
trap cleanup EXIT

# Sparse raw file: claims SIZE bytes but uses ~0 until written to.
truncate -s "${SIZE}" "${disk}"

# Attach it to a free loop device so qemu-img sees a real block device.
loopdev="$(losetup --find --show "${disk}")"

echo "==> Temp file: ${disk} (${SIZE} sparse)"
echo "==> Loop dev:  ${loopdev}"
echo "==> Source:    ${IMG_URL}"
echo "==> Format:    ${FORMAT}"
echo "==> Image:     ${IMAGE}"
echo

set -x
docker run --rm \
    -e IMG_URL="${IMG_URL}" \
    -e DEST_DISK="${loopdev}" \
    -e FORMAT="${FORMAT}" \
    --device "${loopdev}:${loopdev}" \
    "${IMAGE}"
set +x

echo
echo "==> Done."
