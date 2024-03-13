```
quay.io/tinkerbell/actions/image2disk:latest
```

This action will stream a remote disk image (raw) to a block device, and
is mainly used to write cloud images to a disk. It is recommended to use the `qemu-img`
tool to convert disk images into raw, it is also possible to compress the raw images
with tar+gzip to prevent wasted disk space

The below example will stream a raw ubuntu cloud image (converted by qemu-img) and write
it to the block storage disk `/dev/sda`. The raw image is uncompressed in this example.

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

The below example will stream a compressed raw ubuntu cloud image (converted by qemu-img)
and then compressed with gzip to reduce local space.

```
qemu-img convert ubuntu.img ubuntu.raw
gzip ubuntu.raw
```

```yaml
actions:
- name: "stream ubuntu"
  image: quay.io/tinkerbell/actions/image2disk:latest
  timeout: 90
  environment:
      IMG_URL: http://192.168.1.2/ubuntu.tar.gz
      DEST_DISK: /dev/sda
      COMPRESSED: true
```

## Compression format supported:

- bzip2 (`.bzip2`)
- gzip (`.gz`)
- xz (`.xz`)
- xs (`.xs`)
