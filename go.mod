module github.com/tinkerbell/actions

go 1.15

require (
	github.com/containerd/console v0.0.0-20191219165238-8375c3424e4d
	github.com/containerd/containerd v1.4.1-0.20201117152358-0edc412565dc
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/genuinetools/img v0.5.11
	github.com/google/go-cmp v0.4.1
	github.com/klauspost/compress v1.11.12 // indirect
	github.com/mattn/godown v0.0.0-20201027140031-2c7783b24de7
	github.com/moby/buildkit v0.7.2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/yuin/goldmark v1.2.1
	github.com/yuin/goldmark-meta v1.0.0
	go.uber.org/zap v1.16.0
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
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
