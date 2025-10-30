```
quay.io/tinkerbell/actions/oci2disk:latest
```

This action provides the capability to stream a raw (compressed) disk
image from an OCI compliant registry and write this to a block device on a server

To upload a disk image to a compliant OCI registry the [ORAS]() tool is recommended,
as this will simplify the process of creating a new "artifact" that can be used by
`oci2disk`.

* Pushing an OS image to a Harbor Registry with oras *

The below example will push a `debian` image to a registry:

```
# defaults to expected layer media-type of application/vnd.oci.image.layer.v1.tar
oras push 192.168.0.173/test/debian:raw.gz ./debian.raw.gz --insecure
```

We can then use this image by referring too it with teh `IMG_URL` environment variable.

```yaml
actions:
- name: "stream-debian-image"
    image: quay.io/tinkerbell/actions/oci2disk:latest
    timeout: 600
    environment:
      DEST_DISK: /dev/nvme0n1
      IMG_URL: "192.168.0.173/test/debian:raw.gz"
      COMPRESSED: true
```

## Compression format supported:

- bzip2 (`.bzip2`)
- gzip (`.gz`)
- xz (`.xz`)
- xs (`.xs`)
