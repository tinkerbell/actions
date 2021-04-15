#!/bin/sh
set -o errexit
set -o xtrace
# We actually do want EXTRA_ARGS to be split up
#shellcheck disable=SC2086
qemu-img convert "${IMG_URL:?}" -O "${FORMAT:-host_device}" "${DEST_DISK:?}" ${EXTRA_ARGS}
partprobe || true
