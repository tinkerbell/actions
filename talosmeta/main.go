package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/tinkerbell/actions/talosmeta/hardware"
	"github.com/tinkerbell/actions/talosmeta/meta"
	"github.com/tinkerbell/actions/talosmeta/talosnet"
)

const (
	// defaultMetadataPort is the consolidated Tinkerbell HTTP server port that
	// serves /metadata.
	defaultMetadataPort = "7080"
	metadataTimeout     = 60 * time.Second

	linkNamingPredictable = "predictable"
	linkNamingKernel      = "kernel"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("TALOSMETA - Write Talos network configuration to the META partition")

	if err := run(logger); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	disk := os.Getenv("DEST_DISK")
	if disk == "" {
		return errors.New("no block device specified with environment variable [DEST_DISK]")
	}

	doc, err := networkDocument(context.Background(), logger)
	if err != nil {
		return err
	}

	partition, err := meta.Locate(disk)
	if err != nil {
		return err
	}

	logger.Info("Found META partition", "disk", disk, "offset", partition.Offset, "size", partition.Size)

	if err := meta.WriteTag(disk, meta.MetalNetworkPlatformConfig, doc); err != nil {
		return err
	}

	logger.Info("Wrote network configuration to META", "key", fmt.Sprintf("%#x", meta.MetalNetworkPlatformConfig), "bytes", len(doc))
	logger.Info("Network configuration written:\n" + string(doc))

	return nil
}

// networkDocument returns the YAML to store under META key 0xa. In order of
// precedence: the operator-supplied NETWORK_CONFIG document, a Hardware spec
// passed in HARDWARE_SPEC (JSON, as a Workflow template can render it), or the
// Hardware data served by the Tinkerbell metadata service.
func networkDocument(ctx context.Context, logger *slog.Logger) ([]byte, error) {
	if raw := os.Getenv("NETWORK_CONFIG"); raw != "" {
		if _, err := talosnet.Parse([]byte(raw)); err != nil {
			return nil, fmt.Errorf("NETWORK_CONFIG: %w", err)
		}

		logger.Info("Using network configuration from NETWORK_CONFIG")

		return []byte(raw), nil
	}

	namer, err := newNamer(logger)
	if err != nil {
		return nil, err
	}

	spec, source, err := hardwareSpec(ctx, logger)
	if err != nil {
		return nil, err
	}

	if len(spec.Interfaces) == 0 {
		return nil, fmt.Errorf("hardware data from %s has no interfaces; pass the Hardware spec.interfaces through HARDWARE_SPEC, expose them on /metadata, or provide NETWORK_CONFIG", source)
	}

	cfg, err := talosnet.FromHardware(spec, namer)
	if err != nil {
		return nil, fmt.Errorf("mapping hardware data to Talos network configuration: %w", err)
	}

	return cfg.Marshal()
}

// hardwareSpec loads the Hardware spec from HARDWARE_SPEC or the metadata
// service, returning it with a description of where it came from.
func hardwareSpec(ctx context.Context, logger *slog.Logger) (*hardware.Spec, string, error) {
	if raw := os.Getenv("HARDWARE_SPEC"); raw != "" {
		var spec hardware.Spec
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			return nil, "", fmt.Errorf("HARDWARE_SPEC is not valid Hardware spec JSON: %w", err)
		}

		logger.Info("Using hardware data from HARDWARE_SPEC", "interfaces", len(spec.Interfaces))

		return &spec, "HARDWARE_SPEC", nil
	}

	host := os.Getenv("MIRROR_HOST")
	if host == "" {
		return nil, "", errors.New("no hardware data source: set [MIRROR_HOST] for the metadata service, or provide [HARDWARE_SPEC] or [NETWORK_CONFIG]")
	}

	port, ok := os.LookupEnv("METADATA_SERVICE_PORT")
	if !ok {
		port = defaultMetadataPort
	}

	baseURL := "http://" + net.JoinHostPort(host, port)
	logger.Info("Retrieving hardware data", "url", baseURL+"/metadata")

	spec, err := hardware.Fetch(ctx, &http.Client{Timeout: metadataTimeout}, baseURL)
	if err != nil {
		return nil, "", err
	}

	return spec, baseURL, nil
}

func newNamer(logger *slog.Logger) (*talosnet.SysfsNamer, error) {
	mode, ok := os.LookupEnv("LINK_NAMING")
	if !ok || mode == "" {
		mode = linkNamingPredictable
	}

	switch mode {
	case linkNamingPredictable:
		return &talosnet.SysfsNamer{Predictable: true, Logger: logger}, nil
	case linkNamingKernel:
		return &talosnet.SysfsNamer{Predictable: false, Logger: logger}, nil
	default:
		return nil, fmt.Errorf("LINK_NAMING must be %q or %q, got %q", linkNamingPredictable, linkNamingKernel, mode)
	}
}
