# image2disk

```bash
quay.io/tinkerbell/actions/image2disk:latest
```

This Action will stream a remote disk image (raw) to a block device, and
is mainly used to write cloud images to a disk. It is recommended to use the `qemu-img`
tool to convert disk images into raw, it is also possible to compress the raw images
to prevent wasted disk space.

The transfer runs as a pipeline — an HTTP read-ahead ring, a decompressor, a chunker
emitting block-aligned chunks, and a pool of workers issuing `pwrite` — so network,
decompression and disk writes all proceed at once instead of taking turns.

| env var | data type | default value | required | description |
|---------|-----------|---------------|----------|-------------|
| IMG_URL | string | "" | yes | URL of the image to be streamed |
| DEST_DISK | string | "" | yes | Block device to which to write the image |
| COMPRESSED | bool | false | no | Decompress the image before writing it to the disk |
| RETRY_ENABLED | bool | false | no | Retry the Action, using exponential backoff, for the duration specified in `RETRY_DURATION_MINUTES` before failing |
| RETRY_DURATION_MINUTES | int | 10 | no | Duration for which the Action will retry before failing |
| PROGRESS_INTERVAL_SECONDS | int | 3 | no | Interval at which the progress of the image transfer will be logged. `0` disables progress logging |
| TEXT_LOGGING | bool | false | no | Output from the Action will be logged in a more human friendly text format, JSON format is used by default |
| WRITE_CHANGED_BLOCKS_ONLY | bool | false | no | Read each target range before writing it and skip the blocks that already match. See below |
| DIFF_BLOCK_SIZE_BYTES | int | device physical block size | no | Comparison granularity for `WRITE_CHANGED_BLOCKS_ONLY` |
| MEMORY_BUDGET_BYTES | int | 805306368 (768 MiB) | no | Hard cap on the pipeline's buffer pools |
| WRITE_CHUNK_SIZE_BYTES | int | derived from the budget | no | Pipeline chunk size, rounded to a multiple of the device block size |
| WRITE_WORKERS | int | 4, or 1 on rotational media | no | Concurrent `pwrite` workers |
| QUEUE_DEPTH | int | 8 | no | Capacity of the chunk queue feeding the writers |

The below example will stream a raw ubuntu cloud image (converted by qemu-img) and write
it to the block storage disk `/dev/sda`. The raw image is uncompressed in this example.

```bash
qemu-img convert ubuntu.img ubuntu.raw
```

```yaml
actions:
- name: "stream ubuntu"
  image: quay.io/tinkerbell/actions/image2disk:latest
  timeout: 90
  environment:
      IMG_URL: http://192.168.1.2/ubuntu.raw
      DEST_DISK: /dev/sda
      COMPRESSED: false
```

The below example will stream a compressed raw ubuntu cloud image (converted by qemu-img) and write
it to the block storage disk `/dev/sda`. The raw image is compressed with zstd in this example.

```bash
qemu-img convert ubuntu.img ubuntu.raw
zstd ubuntu.raw
```

```yaml
actions:
- name: "stream ubuntu"
  image: quay.io/tinkerbell/actions/image2disk:latest
  timeout: 90
  environment:
      IMG_URL: http://192.168.1.2/ubuntu.raw.zst
      DEST_DISK: /dev/sda
      COMPRESSED: true
```

## Supported compression formats

The format is chosen from the extension of the image URL's *path*, so a presigned link
with a query string (`.../ubuntu.raw.zst?X-Amz-Signature=...`) is detected correctly.

| format | extensions | decodes across cores? |
|--------|------------|-----------------------|
| zstd | `.zst`, `.zs` | partly — the decoder pipelines across about three cores |
| bzip2 | `.bz2`, `.bzip2` | yes — bzip2 blocks are independent, so every core is used |
| gzip | `.gz` | **no** |
| xz | `.xz` | **no** |

Two caveats worth knowing before choosing a format:

- **gzip cannot be decompressed in parallel.** DEFLATE is a single sliding window and is
  strictly serial, so gzip tops out at roughly 250–400 MB/s on one core no matter how many
  cores the machine has. If the logs show `decodeRate` far below `readRate` and `writeRate`,
  the gzip decoder is the bottleneck and the fix is to republish the image as zstd.
- **xz is single-threaded here and slow** (roughly 30–40 MB/s). The xz container *can* be
  decoded in parallel when produced with `xz -T0`, but that needs liblzma via cgo, which this
  build does not use.

Note that nothing untars: the image must be a raw disk image, optionally compressed. A
`.tar.gz` will have its gzip layer stripped and the tar stream itself written to the disk.

## Only writing changed blocks

With `WRITE_CHANGED_BLOCKS_ONLY=true` the action reads each target range from the device
before writing it, compares it a block at a time, and writes only the runs of blocks that
actually differ. Consecutive differing blocks are coalesced into a single `pwrite`.

```yaml
actions:
- name: "re-image ubuntu"
  image: quay.io/tinkerbell/actions/image2disk:latest
  timeout: 90
  environment:
      IMG_URL: http://192.168.1.2/ubuntu.raw.zst
      DEST_DISK: /dev/nvme0n1
      COMPRESSED: true
      WRITE_CHANGED_BLOCKS_ONLY: true
```

When it helps:

- Re-imaging a machine that already holds most of the same content.
- Flash media, where reads are several times cheaper than writes, and where avoiding
  writes saves erase cycles.

When it does not:

- **A blank or entirely different disk.** Every block differs, so the read pass is pure
  overhead and the total IO is roughly doubled.
- **Anywhere the bottleneck is upstream.** The image is still downloaded and decompressed
  in full; only the writes are avoided. If the network or the decompressor is the limit,
  this mode saves no wall clock at all.

The mode needs read access, so the device is opened `O_RDWR` rather than `O_WRONLY`. If the
destination turns out not to be readable, the action logs a warning and falls back to
unconditional writes for the whole run rather than failing. A read that fails partway through
is likewise never treated as "unchanged": that chunk is written unconditionally and the
failure count is reported at the end.

## Failure reporting

A truncated download now fails the action. Previously the copy ignored `io.EOF` and
`io.ErrUnexpectedEOF`, so a transfer cut short reported success and the machine went on to
boot a corrupt filesystem. Compressed streams are validated by their own trailers (gzip CRC,
zstd checksum) and uncompressed ones are checked against the advertised `Content-Length`.
Combined with `RETRY_ENABLED`, a partial transfer is now retried rather than accepted.

The HTTP client also has a response-header timeout and a stall watchdog, so a server that
accepts the connection and then goes quiet fails the attempt instead of hanging forever.
