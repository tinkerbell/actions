REGISTRY?=quay.io
REPOSITORY?=tinkerbell-actions
NAME?=$(shell awk '/^name: /{print $$2}' README.md)
VERSION?=$(shell awk '/^version: /{print $$2}' README.md)
IMAGE=$(REGISTRY)/$(REPOSITORY)/$(NAME):$(VERSION)

image:
	docker buildx build --load --platform linux/amd64 -t $(IMAGE) .

push: image
	docker push $(IMAGE)

# Manual tests specific to qemuimg2disk
HTTPS_IMAGE_URL=https://cloud-images.ubuntu.com/daily/server/focal/current/focal-server-cloudimg-amd64.img
test-https: image
	docker run --rm -it -e IMG_URL=$(HTTPS_IMAGE_URL) -e DEST_DISK=/deleteme -e FORMAT=qcow2 -e EXTRA_ARGS="-p" $(IMAGE)
