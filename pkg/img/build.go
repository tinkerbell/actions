package img

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/console"
	"github.com/containerd/containerd/namespaces"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/docker/distribution/reference"
	"github.com/genuinetools/img/client"
	controlapi "github.com/moby/buildkit/api/services/control"
	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Disclaimer: The code is heavily based on img, hence I omitted
// some features like labels and additional tags for now.
// Reference: https://github.com/genuinetools/img

const (
	defaultBackend        = "auto"
	defaultDockerfileName = "Dockerfile"
)

// BuildConfig configures the required parameters for img
// to build a cross-platform container image.
type BuildConfig struct {
	Context    string
	Dockerfile string
	Tag        string
	Platforms  string
	Target     string
	Push       bool
	NoConsole  bool
}

// Build programmatically runs img, which under the hood uses buildkit to build a Tinkerbell action.
func Build(config *BuildConfig) error {
	var err error

	// Check if a build context is set.
	if config.Context == "" {
		config.Context = "."
	}

	// Check if the build context should be read from stdin.
	if config.Context == "-" {
		return errors.New("reading from stdin is currently unsupported")
	}

	// Set the dockerfile path as the default if one was not given.
	if config.Dockerfile == "" {
		config.Dockerfile, err = securejoin.SecureJoin(config.Context, defaultDockerfileName)
		if err != nil {
			return err
		}
	}

	// Check if input should be read from stdin.
	if config.Dockerfile == "-" {
		return errors.New("reading from stdin is currently unsupported")
	}

	// Check if the container tag is valid.
	config.Tag, err = validateTag(config.Tag)
	if err != nil {
		return err
	}

	// Get the state directory.
	stateDir := stateDirectory()

	// Create the client.
	c, err := client.New(stateDir, defaultBackend, map[string]string{
		"context":    config.Context,
		"dockerfile": filepath.Dir(config.Dockerfile),
	})
	if err != nil {
		return err
	}
	defer c.Close()

	// Create the frontend attrs.
	frontendAttrs := map[string]string{
		// We use the base for filename here because we already set up the local dirs which sets the path in createController.
		"filename": filepath.Base(config.Dockerfile),
		"target":   config.Target,
		"platform": config.Platforms,
	}

	// Create the context.
	ctx := appcontext.Context()
	sess, sessDialer, err := c.Session(ctx)
	if err != nil {
		return err
	}
	id := identity.NewID()
	ctx = session.NewContext(ctx, sess.ID())
	ctx = namespaces.WithNamespace(ctx, "buildkit")
	eg, ctx := errgroup.WithContext(ctx)

	// Prepare the exporter
	out := bkclient.ExportEntry{
		Type: bkclient.ExporterImage,
		Attrs: map[string]string{
			"name": config.Tag,
		},
	}
	if config.Push {
		out.Attrs["push"] = "true"
	}

	ch := make(chan *controlapi.StatusResponse)
	eg.Go(func() error {
		return sess.Run(ctx, sessDialer)
	})

	// Configure buildkit's export-cache.
	exportCache := []*controlapi.CacheOptionsEntry{
		{
			Type: "inline",
		},
	}

	// Configure buildkit's import-cache.
	importCache := []*controlapi.CacheOptionsEntry{
		{
			Type: "registry",
			Attrs: map[string]string{
				"ref": config.Tag,
			},
		},
	}

	// Configure the imported cache for the frontend.
	importCacheMarshalled, err := json.Marshal(importCache)
	if err != nil {
		return errors.Wrap(err, "failed to marshal import-cache")
	}
	frontendAttrs["cache-imports"] = string(importCacheMarshalled)

	// Solve the dockerfile.
	eg.Go(func() error {
		defer sess.Close()
		return c.Solve(ctx, &controlapi.SolveRequest{
			Ref:           id,
			Session:       sess.ID(),
			Exporter:      out.Type,
			ExporterAttrs: out.Attrs,
			Frontend:      "dockerfile.v0",
			FrontendAttrs: frontendAttrs,
			Cache: controlapi.CacheOptions{
				Exports: exportCache,
				Imports: importCache,
			},
		}, ch)
	})
	eg.Go(func() error {
		return showProgress(ch, config.NoConsole)
	})

	err = eg.Wait()
	return err
}

// validateTag checks if the given image name can be resolved, and ensures the latest tag is added if it is missing.
func validateTag(repo string) (string, error) {
	named, err := reference.ParseNormalizedNamed(repo)
	if err != nil {
		return "", err
	}

	// Add the latest tag if they did not provide one.
	return reference.TagNameOnly(named).String(), nil
}

// stateDirectory gets a state directory to store the build state in.
func stateDirectory() string {
	//  pam_systemd sets XDG_RUNTIME_DIR but not other dirs.
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		dirs := strings.Split(xdgDataHome, ":")
		return filepath.Join(dirs[0], "img")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".local", "share", "img")
	}
	return "/tmp/img"
}

func showProgress(ch chan *controlapi.StatusResponse, noConsole bool) error {
	displayCh := make(chan *bkclient.SolveStatus)
	go func() {
		for resp := range ch {
			s := bkclient.SolveStatus{}
			for _, v := range resp.Vertexes {
				s.Vertexes = append(s.Vertexes, &bkclient.Vertex{
					Digest:    v.Digest,
					Inputs:    v.Inputs,
					Name:      v.Name,
					Started:   v.Started,
					Completed: v.Completed,
					Error:     v.Error,
					Cached:    v.Cached,
				})
			}
			for _, v := range resp.Statuses {
				s.Statuses = append(s.Statuses, &bkclient.VertexStatus{
					ID:        v.ID,
					Vertex:    v.Vertex,
					Name:      v.Name,
					Total:     v.Total,
					Current:   v.Current,
					Timestamp: v.Timestamp,
					Started:   v.Started,
					Completed: v.Completed,
				})
			}
			for _, v := range resp.Logs {
				s.Logs = append(s.Logs, &bkclient.VertexLog{
					Vertex:    v.Vertex,
					Stream:    int(v.Stream),
					Data:      v.Msg,
					Timestamp: v.Timestamp,
				})
			}
			displayCh <- &s
		}
		close(displayCh)
	}()
	var c console.Console
	if !noConsole {
		if cf, err := console.ConsoleFromFile(os.Stderr); err == nil {
			c = cf
		}
	}
	return progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stderr, displayCh)
}
