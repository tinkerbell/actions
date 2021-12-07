---
slug: archive2disk
name: archive2disk
tags: disk
maintainers: Dan Finneran <daniel.finneran@gmail.com>
description: "This action will stream a compressed filesystem archive into archive2disk
where it will mount a block device and uncompress this archive to a destination path on
it's filesystem. Usually the `DEST_PATH` should be `/` as we normally will be writing a
root filesystem, however it can be used to stream application data to a different area on
disk (such as preloading container images, or web server assets)."
version: v1.0.0
createdAt: "2021-02-10T12:41:45.14Z"
---

The below example will stream a compressed root filesystem into archive2disk, where it
will be written to the filesystem on the block device `/dev/sda3`.

Archive types supported:

-   `tar`: standard tar archive
-   `targz`: A gzipped tar archive (compressed)

Optional SHASUM verification on the TAR file (not the tar.gz file), see usage examples below and more details at
github.com/opencontainers/go-digest

```yaml
actions:
    - name: "expand-ubuntu-filesystem-to-root"
      image: quay.io/tinkerbell-actions/archive2disk:v1.0.0
      timeout: 90
      environment:
          ARCHIVE_URL: http://192.168.1.2/ubuntu.tar.gz
          ARCHIVE_TYPE: targz
          DEST_DISK: /dev/sda3
          FS_TYPE: ext4
          DEST_PATH: /
```
```yaml
actions:
    - name: "expand-ubuntu-filesystem-to-root"
      image: quay.io/tinkerbell-actions/archive2disk:v1.0.0
      timeout: 90
      environment:
          ARCHIVE_URL: http://192.168.1.2/ubuntu.tar.gz
          ARCHIVE_TYPE: targz
          TARFILE_CHECKSUM: sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2
          DEST_DISK: /dev/sda3
          FS_TYPE: ext4
          DEST_PATH: /
```
```yaml
actions:
    - name: "expand-ubuntu-filesystem-to-root"
      image: quay.io/tinkerbell-actions/archive2disk:v1.0.0
      timeout: 90
      environment:
          ARCHIVE_URL: http://192.168.1.2/ubuntu.tar.gz
          ARCHIVE_TYPE: targz
          TARFILE_CHECKSUM: sha512:0e3e75234abc68f4378a86b3f4b32a198ba301845b0cd6e50106e874345700cc6663a86c1ea125dc5e92be17c98f9a0f85ca9d5f595db2012f7cc3571945c123
          DEST_DISK: /dev/sda3
          FS_TYPE: ext4
          DEST_PATH: /
```