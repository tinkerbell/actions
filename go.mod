module github.com/tinkerbell/actions

go 1.15

require (
	github.com/Microsoft/hcsshim/test v0.0.0-20210408205431-da33ecd607e1 // indirect
	github.com/containerd/console v1.0.1
	github.com/containerd/containerd v1.5.0-beta.4
	github.com/containerd/continuity v0.0.0-20201208142359-180525291bb7 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/cli v20.10.5+incompatible // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/genuinetools/img v0.5.11
	github.com/google/go-cmp v0.5.2
	github.com/mattn/godown v0.0.0-20201027140031-2c7783b24de7
	github.com/moby/buildkit v0.7.2
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/yuin/goldmark v1.2.1
	github.com/yuin/goldmark-meta v1.0.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	gotest.tools/v3 v3.0.3 // indirect
)

replace (
	// containerd: needs replacement because img references unrelease version of containerd
	github.com/containerd/containerd => github.com/containerd/containerd v1.4.3
	// estargz: needs this replace because stargz-snapshotter git repo has two go.mod modules.
	github.com/containerd/stargz-snapshotter/estargz => github.com/containerd/stargz-snapshotter/estargz v0.0.0-20201217071531-2b97b583765b
	// docker: needs replacement because it is called moby now
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20210128214336-420b1d36250f+incompatible
	// protobuf: corresponds to containerd
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.5
	github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe
	github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
	// genproto: corresponds to containerd
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63
)
