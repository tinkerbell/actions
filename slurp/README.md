```
quay.io/tinkerbell/actions/slurp:latest
```

This action will stream the contents of a block device to a remote server
allowing a user to easily make a copy of a remote server. This newly created disk image
can be replicated to easily provision other servers.

Slurp will read directly from a block device, it can compress this stream and will
use the HTTP protocol to transmit the raw data to a listening server. The remote
server needs to be able to handle `multipart` data (due to the size) and reform this
to a disk image locally.

The below example will stream a raw ubuntu installed on `/dev/sda` and uplopad it to
a remote server.

```yaml
actions:
- name: "slurp_ubuntu"
  image: quay.io/tinkerbell/actions/slurp:latest
  timeout: 90
  environment:
    DEST_URL: http://192.168.1.2:3000/image
    SOURCE_DISK: /dev/sda
    IMG_NAME: ubuntu_wordpress
    COMPRESSED: false
```

**Note** the `DEST_URL`, the slurp action has a server that can be used with this action.
It can be started with `go run server/main.go -port 3000`, which will create a web server
that has a handler on the url `/image` to manage images that are uploaded too it.
