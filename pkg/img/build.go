package img

import (
	"context"

	"os"
	"path/filepath"

	"github.com/containerd/console"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const (
	defaultBackend        = "auto"
	defaultDockerRegistry = "https://index.docker.io/v1/"
	defaultDockerfileName = "Dockerfile"
)

// BuildConfig configures the required parameters for img
// to build a cross-platform container image.
type BuildConfig struct {
	Context      string
	Dockerfile   string
	Tag          string
	Platforms    string
	Target       string
	BuildKitAddr string
	Push         bool
	NoCache      bool
}

// Build programmatically runs img, which under the hood uses buildkit to build a Tinkerbell action.
func Build(ctx context.Context, config *BuildConfig) error {
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

	localDirs := map[string]string{
		"context":    config.Context,
		"dockerfile": filepath.Dir(config.Dockerfile),
	}
	attachable := []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)}

	// Check if the container tag is valid.
	config.Tag, err = validateTag(config.Tag)
	if err != nil {
		return err
	}

	// Create the client.
	c, err := client.New(ctx, config.BuildKitAddr, client.WithFailFast())
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
	if config.NoCache {
		frontendAttrs["no-cache"] = ""
	}

	eg, ctx := errgroup.WithContext(ctx)

	// Prepare the exporter
	out := client.ExportEntry{
		Type: client.ExporterImage,
		Attrs: map[string]string{
			"name": config.Tag,
		},
	}
	if config.Push {
		out.Attrs["push"] = "true"
	}

	ch := make(chan *client.SolveStatus)

	// Solve the dockerfile.
	eg.Go(func() error {
		solveOpt := client.SolveOpt{
			Exports:       []client.ExportEntry{out},
			Session:       attachable,
			LocalDirs:     localDirs,
			Frontend:      "dockerfile.v0",
			FrontendAttrs: frontendAttrs,
		}

		_, err := c.Solve(ctx, nil, solveOpt, ch)
		return err
	})
	eg.Go(func() error {
		var c console.Console
		if cn, err := console.ConsoleFromFile(os.Stderr); err == nil {
			c = cn
		}
		// not using shared context to not disrupt display but let is finish reporting errors
		return progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stdout, ch)
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
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
