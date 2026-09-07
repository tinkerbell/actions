```
quay.io/tinkerbell/actions/oci2disk:latest
```

This action provides the capability to stream a raw (compressed) disk
image from an OCI compliant registry and write this to a block device on a server

To upload a disk image to a compliant OCI registry the [ORAS](https://oras.land) tool is recommended,
as this will simplify the process of creating a new "artifact" that can be used by
`oci2disk`.

* Pushing an OS image to a Harbor Registry with oras *

The below example will push a `debian` image to a registry:

```
# defaults to expected layer media-type of application/vnd.oci.image.layer.v1.tar
oras push 192.168.0.173/test/debian:raw.gz ./debian.raw.gz --insecure
```

We can then use this image by referring to it with the `IMG_URL` environment variable.

```yaml
actions:
- name: "stream-debian-image"
    image: quay.io/tinkerbell/actions/oci2disk:latest
    timeout: 600
    environment:
      DEST_DISK: /dev/nvme0n1
      IMG_URL: "192.168.0.173/test/debian:raw.gz"
      # optional fields for registry authentication
      REGISTRY_USERNAME: "foo"
      REGISTRY_PASSWORD: "bar"
      # optional field to skip TLS verification (defaults to false)
      SKIP_VERIFY: "true"
```

## Environment Variables:

- `DEST_DISK`: Target block device to write the image to (required)
- `IMG_URL`: OCI image reference, `<name:tag|name@digest>` (required)
- `REGISTRY_USERNAME` / `REGISTRY_PASSWORD`: registry credentials (optional, anonymous pull when unset)
- `SKIP_VERIFY`: set to `true` to skip TLS certificate verification (optional, defaults to `false`)

The platform (`linux/amd64`, `linux/arm64`, ...) is selected automatically from the
architecture the action runs on when the image is a multi-platform index.

## Compression format supported:

The compression format is detected from the file name recorded in the layer's
`org.opencontainers.image.title` annotation, which `oras push` sets to the name of
the pushed file.

- bzip2 (`.bzip2`, `.bz2`)
- gzip (`.gz`)
- xz (`.xz`)
- zstd (`.zst`, `.zs`)
