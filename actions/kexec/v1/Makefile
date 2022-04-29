REGISTRY?=quay.io
REPOSITORY?=tinkerbell-actions/kexec
VERSION?=v1.0.0
IMAGE=$(REGISTRY)/$(REPOSITORY):$(VERSION)

image:
	docker buildx build --load --platform linux/amd64 -t $(IMAGE) .

push: image
	docker push $(IMAGE)

