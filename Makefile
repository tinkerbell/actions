ci: bin/gofumpt
	./hack/ci-check.sh

artifacthub/gen-manifests:
	./hack/generate-artifacthub-manifests.sh

include lint.mk
include rules.mk
