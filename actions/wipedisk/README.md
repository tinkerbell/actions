---
slug: disk-wipe
name: Disk Wipe
tags: disk
maintainers: Dan Finneran <email@email.com>
description: "This action wipes an entire disk. A common use for it is at the
beginning of the workflow, to be sure that you are starting from a cleaned
environment"
version: v0.1.0
createdAt: "2016-06-20T12:41:45.14Z"
---

Wipe an entire disk. This is not a joke! All the data in your hard drive or SSD
will go away. Be prepared.

```
  actions:
  - name: "disk-wipe"
    image: quay.io/tinkerbell-action/disk-wipe:v0.1.0
    timeout: 90
```
