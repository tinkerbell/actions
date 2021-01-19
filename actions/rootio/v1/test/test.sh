#!/bin/bash

echo "Creating a 4GB file to use as a block device"

dd if=/dev/zero of=./disk bs=1M count=4096

ls -lah ./disk
