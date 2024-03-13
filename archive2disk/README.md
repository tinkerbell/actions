```
quay.io/tinkerbell/actions/archive2disk:latest
```

This action will stream a compressed filesystem archive into archive2disk
where it will mount a block device and uncompress this archive to a destination path on
it's filesystem. Usually the `DEST_PATH` should be `/` as we normally will be writing a
root filesystem, however it can be used to stream application data to a different area on
disk (such as preloading container images, or web server assets).

The below example will stream a compressed root filesystem into archive2disk, where it
will be written to the filesystem on the block device `/dev/sda3`.

Archive types supported:

-   `tar`: standard tar archive
-   `targz`: A gzipped tar archive (compressed)

Environment Variables:
  - ARCHIVE_URL (Required) Specify the compressed file URL
  - ARCHIVE_TYPE (Required) Specify the type of archive.  Supported archive types are TAR and TARGZ
  - TARFILE_CHECKSUM (Required, overridable) Specify the checksum of the TAR file
    The format for specifying the checksum is algorithm:hash. See examples below, more informaiton can be found at github.com/opencontainers/go-digest
  - INSECURE_NO_TARFILE_CHECKSUM_VERIFICATION (optional) Set to true to skip the check for the TARFILE_CHECKSUM environment variable
  - DEST_DISK (Required) Specify the block device that will get mounted and where the archive will uncompress
  - FS_TYPE (Required) Specify the file system type of DEST_DISK
  - DEST_PATH  (Required) Specify the destination path location where the archive will uncompress
  - HTTP_CLIENT_TIMEOUT_MINUTES (Optional) Specify the HTTP timeout in minutes when accessing the ARCHIVE_URL.  The default is 5 minutes.

```yaml
actions:
- name: "expand-ubuntu-filesystem-to-root"
  image: quay.io/tinkerbell/actions/archive2disk:latest
  timeout: 90
  environment:
      ARCHIVE_URL: http://192.168.1.2/ubuntu.tar.gz
      ARCHIVE_TYPE: targz
      TARFILE_CHECKSUM: sha256:f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2
      DEST_DISK: /dev/sda3
      FS_TYPE: ext4
      DEST_PATH: /
      HTTP_CLIENT_TIMEOUT_MINUTES: 1
```
```yaml
actions:
- name: "expand-ubuntu-filesystem-to-root"
  image: quay.io/tinkerbell/actions/archive2disk:latest
  timeout: 90
  environment:
      ARCHIVE_URL: http://192.168.1.2/ubuntu.tar.gz
      ARCHIVE_TYPE: targz
      TARFILE_CHECKSUM: sha512:0e3e75234abc68f4378a86b3f4b32a198ba301845b0cd6e50106e874345700cc6663a86c1ea125dc5e92be17c98f9a0f85ca9d5f595db2012f7cc3571945c123
      DEST_DISK: /dev/sda3
      FS_TYPE: ext4
      DEST_PATH: /
```
```yaml
actions:
- name: "expand-ubuntu-filesystem-to-root"
  image: quay.io/tinkerbell/actions/archive2disk:latest
  timeout: 90
  environment:
      ARCHIVE_URL: http://192.168.1.2/ubuntu.tar.gz
      ARCHIVE_TYPE: targz
      DEST_DISK: /dev/sda3
      FS_TYPE: ext4
      DEST_PATH: /
      INSECURE_NO_TARFILE_CHECKSUM_VERIFICATION: true
```
