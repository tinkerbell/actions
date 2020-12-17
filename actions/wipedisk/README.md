---
slug: disk-wipe
name: Disk Wipe
tags: disk
maintainers: Dan Finneran <email@email.com>
description: "this action wipes an entire disk"
version: v0.1.0
createdAt: "January 2, 2016"
---

Wipe an entire disk. This is not a joke! All the data in your hard drive or SSD
will go away. Be prepared.

```
  actions:
  - name: "disk-wipe"
    image: quay.io/tinkerbell-action/disk-wipe:v0.1.0
    timeout: 90
```
