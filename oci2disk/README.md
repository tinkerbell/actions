```
quay.io/tinkerbell/actions/oci2disk:latest
```

This action provides the capability to stream a raw (compressed) disk
image from an OCI compliant registry and write this to a block device on a server

To upload a disk image to a compliant OCI registry the [ORAS]() tool is recommended,
as this will simplify the process of creating a new "artifact" that can be used by
`oci2disk`.

* Pushing an OS image to a Harbor Registry with oras *

The below example will push a gzip-compressed `debian` image to a registry:

```bash
# Push the compressed image (the filename doesn't need to include the compression extension)
oras push 192.168.0.173/test/debian:latest ./debian.raw.gz --insecure
```

We can then use this image by referring to it with the `IMG_URL` environment variable
and specifying the compression algorithm via `COMPRESSED`:

```yaml
actions:
- name: "stream-debian-image"
  image: quay.io/tinkerbell/actions/oci2disk:latest
  timeout: 600
  environment:
    DEST_DISK: /dev/nvme0n1
    IMG_URL: "192.168.0.173/test/debian:latest"
    COMPRESSED: gz
```

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `DEST_DISK` | Target block device (e.g., `/dev/nvme0n1`) | Yes |
| `IMG_URL` | OCI image reference (e.g., `registry/repo:tag`) | Yes |
| `COMPRESSED` | Compression algorithm (see below) | No |

## Compression Formats Supported

| Value | Format |
|-------|--------|
| `gz` or `gzip` | gzip |
| `xz` | xz |
| `zstd` or `zst` | Zstandard |
| `bzip2` | bzip2 |
| `true` | Auto-detect from image tag extension (backward compatible) |
| (empty) | No compression |

### Recommended Usage

Specify the compression algorithm explicitly:

```yaml
COMPRESSED: gz
```

### Backward Compatibility

For backward compatibility, `COMPRESSED: true` is still supported and will detect
the compression format from the image tag extension (e.g., `image:tag.gz` → gzip).
However, explicitly specifying the compression algorithm is preferred as it doesn't
require encoding the compression type in the image tag.
