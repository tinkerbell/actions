---
slug: archive2disk
name: archive2disk
tags: disk
maintainers: Dan Finneran <daniel.finneran@gmail.com>
description: "This action will stream a compressed filesystem archive into archive2disk
where it will mount a block device and uncompress this archive to a destination path on
<<<<<<< HEAD
it's filesystem. Usually the `DEST_PATH` should be "/" as we normally will be writing a 
=======
it's filesystem. Usually the `DEST_PATH` should be `/` as we normally will be writing a
>>>>>>> upstream/main
root filesystem, however it can be used to stream application data to a different area on
disk (such as preloading container images, or web server assets)."
version: v1.0.0
createdAt: "2021-02-10T12:41:45.14Z"
---

The below example will stream a compressed root filesystem into archive2disk, where it
will be written to the filesystem on the block device `/dev/sda3`.

Archive types supported:
<<<<<<< HEAD
- `tar`: standard tar archive
- `targz`: A gzipped tar archive (compressed)
=======

-   `tar`: standard tar archive
-   `targz`: A gzipped tar archive (compressed)
>>>>>>> upstream/main

```yaml
actions:
    - name: "expand ubuntu filesystem to root"
      image: quay.io/tinkerbell-actions/archive2disk:v1.0.0
      timeout: 90
      environment:
<<<<<<< HEAD
          ARCHIVE_URL: 192.168.1.2/ubuntu.tar.gz
=======
          ARCHIVE_URL: http://192.168.1.2/ubuntu.tar.gz
>>>>>>> upstream/main
          ARCHIVE_TYPE: targz
          DEST_DISK: /dev/sda3
          FS_TYPE: ext4
          DEST_PATH: /
<<<<<<< HEAD
```
=======
```
>>>>>>> upstream/main
