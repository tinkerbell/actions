ifeq ($(origin GOBIN), undefined)
GOBIN := ${PWD}/bin
export GOBIN
PATH := ${GOBIN}:${PATH}
export PATH
endif


toolLibs = golang.org/x/tools/cmd/goimports mvdan.cc/gofumpt
toolBins = $(addprefix bin/,$(notdir $(toolLibs)))

# installs cli tools
$(toolBins):
	go install $(filter %$(notdir $@),$(toolLibs))@latest
