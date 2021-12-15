ci:
	./hack/ci-check.sh

artifacthub/gen-manifests:
	./hack/generate-artifacthub-manifests.sh

include rules.mk
