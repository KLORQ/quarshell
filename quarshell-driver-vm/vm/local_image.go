package vm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/KLORQ/quarshell-driver/podman"
	"github.com/containerd/errdefs"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	engine "github.com/moby/moby/client"
)

const localOCIImageArchiveMaxBytes = rootfsRegistryMaxCompressedBytes

type localImageMetadata struct {
	ID           string
	OS           string
	Architecture string
	Config       v1.Config
}

type localImageEngine interface {
	InspectImage(context.Context, string) (localImageMetadata, error)
	SaveImage(context.Context, string, io.Writer) error
}

type dockerLocalImageEngine struct{ client *engine.Client }

func (e dockerLocalImageEngine) InspectImage(ctx context.Context, reference string) (localImageMetadata, error) {
	image, err := e.client.ImageInspect(ctx, reference)
	if err != nil {
		return localImageMetadata{}, err
	}
	if image.Config == nil {
		return localImageMetadata{}, fmt.Errorf("local image %q has no OCI configuration", reference)
	}
	config := v1.Config{
		Entrypoint: append([]string(nil), image.Config.Entrypoint...),
		Cmd:        append([]string(nil), image.Config.Cmd...),
		Env:        append([]string(nil), image.Config.Env...),
		User:       image.Config.User,
		WorkingDir: image.Config.WorkingDir,
	}
	return localImageMetadata{ID: image.ID, OS: image.Os, Architecture: image.Architecture, Config: config}, nil
}

func (e dockerLocalImageEngine) SaveImage(ctx context.Context, imageID string, destination io.Writer) error {
	archive, err := e.client.ImageSave(ctx, []string{imageID})
	if err != nil {
		return err
	}
	defer archive.Close()
	_, err = io.Copy(destination, archive)
	return err
}

func newLocalImageEngines() []localImageEngine {
	var engines []localImageEngine
	if cli, err := engine.New(engine.FromEnv); err == nil {
		engines = append(engines, dockerLocalImageEngine{client: cli})
	}
	// The Docker-compatible engine can be Podman even when DOCKER_HOST is not
	// configured. Resolve its user socket only after trying Docker's defaults.
	if strings.TrimSpace(os.Getenv("DOCKER_HOST")) == "" && strings.TrimSpace(os.Getenv("CONTAINER_HOST")) == "" {
		if host, err := podman.ResolveHost(); err == nil {
			if cli, err := engine.New(engine.WithHost(host)); err == nil {
				engines = append(engines, dockerLocalImageEngine{client: cli})
			}
		}
	}
	return engines
}

func closeLocalImageEngines(engines []localImageEngine) {
	for _, candidate := range engines {
		if docker, ok := candidate.(dockerLocalImageEngine); ok && docker.client != nil {
			_ = docker.client.Close()
		}
	}
}

func requiredLocalImageReference(reference string) bool {
	return strings.HasPrefix(reference, "openshell/sandbox-from:")
}

// prepareLocalOCIImage first checks a local Docker/Podman image. The saved
// archive is bounded and selected by immutable Engine image ID, so a tag move
// between inspect and export cannot silently change the rootfs.
func prepareLocalOCIImage(ctx context.Context, reference, cacheDir, formatterPath, guestAgentPath string, cacheMaxBytes int64, engines []localImageEngine) (preparedRootfsImage, resolvedOCIImage, bool, error) {
	if ctx == nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, false, fmt.Errorf("local image context is nil")
	}
	required := requiredLocalImageReference(reference)
	if engines == nil {
		engines = newLocalImageEngines()
		defer closeLocalImageEngines(engines)
	}
	var inspectErr error
	for _, candidate := range engines {
		if err := ctx.Err(); err != nil {
			return preparedRootfsImage{}, resolvedOCIImage{}, false, err
		}
		metadata, err := candidate.InspectImage(ctx, reference)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				inspectErr = err
			}
			continue
		}
		platform, err := vmOCIPlatform()
		if err != nil {
			return preparedRootfsImage{}, resolvedOCIImage{}, false, err
		}
		if metadata.OS != platform.OS || metadata.Architecture != platform.Architecture {
			message := fmt.Sprintf("local container image %q is %s/%s; VM requires %s", reference, metadata.OS, metadata.Architecture, platform.String())
			if required {
				return preparedRootfsImage{}, resolvedOCIImage{}, false, fmt.Errorf("vm driver: %s", message)
			}
			return preparedRootfsImage{}, resolvedOCIImage{}, false, nil
		}
		if _, err := v1.NewHash(metadata.ID); err != nil {
			return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("local container image %q has invalid immutable ID: %w", reference, err)
		}
		return prepareLocalOCIImageFromEngine(ctx, candidate, metadata, reference, platform, cacheDir, formatterPath, guestAgentPath, cacheMaxBytes)
	}
	if required {
		if inspectErr != nil {
			return preparedRootfsImage{}, resolvedOCIImage{}, false, fmt.Errorf("vm driver: required local image %q is unavailable: %w", reference, inspectErr)
		}
		return preparedRootfsImage{}, resolvedOCIImage{}, false, fmt.Errorf("vm driver: required local image %q is not present in Docker or Podman", reference)
	}
	return preparedRootfsImage{}, resolvedOCIImage{}, false, nil
}

func resolveLocalOCIImageConfig(ctx context.Context, reference, expectedID string, platform v1.Platform, engines []localImageEngine) (v1.Config, error) {
	if engines == nil {
		engines = newLocalImageEngines()
		defer closeLocalImageEngines(engines)
	}
	var inspectErr error
	for _, candidate := range engines {
		metadata, err := candidate.InspectImage(ctx, reference)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				inspectErr = err
			}
			continue
		}
		if metadata.ID != expectedID {
			return v1.Config{}, fmt.Errorf("local image %q now resolves to %s; durable VM state requires %s", reference, metadata.ID, expectedID)
		}
		if metadata.OS != platform.OS || metadata.Architecture != platform.Architecture {
			return v1.Config{}, fmt.Errorf("local image %q platform changed to %s/%s; durable VM state requires %s", reference, metadata.OS, metadata.Architecture, platform.String())
		}
		return metadata.Config, nil
	}
	if inspectErr != nil {
		return v1.Config{}, fmt.Errorf("inspect local image %q during VM recovery: %w", reference, inspectErr)
	}
	return v1.Config{}, fmt.Errorf("local image %q is no longer available during VM recovery", reference)
}

func prepareLocalOCIImageFromEngine(ctx context.Context, engine localImageEngine, metadata localImageMetadata, reference string, platform v1.Platform, cacheDir, formatterPath, guestAgentPath string, cacheMaxBytes int64) (preparedRootfsImage, resolvedOCIImage, bool, error) {
	archiveFile, err := os.CreateTemp("", "cautem-vm-local-image-*.tar")
	if err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("create temporary local image archive: %w", err)
	}
	archivePath := archiveFile.Name()
	keepArchive := false
	defer func() {
		_ = archiveFile.Close()
		if !keepArchive {
			_ = os.Remove(archivePath)
		}
	}()
	if err := archiveFile.Chmod(0o600); err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("secure local image archive: %w", err)
	}
	bounded := &boundedWriter{writer: archiveFile, limit: localOCIImageArchiveMaxBytes}
	if err := engine.SaveImage(ctx, metadata.ID, bounded); err != nil {
		if errors.Is(err, errBoundedWriterLimit) {
			return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("local image archive exceeds %d byte limit", localOCIImageArchiveMaxBytes)
		}
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("save local image %q by immutable ID: %w", reference, err)
	}
	if err := ctx.Err(); err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, err
	}
	if err := archiveFile.Sync(); err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("sync local image archive: %w", err)
	}
	image, err := tarball.ImageFromPath(archivePath, nil)
	if err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("read Docker-compatible image archive for %q: %w", reference, err)
	}
	imageConfig, err := image.ConfigFile()
	if err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("read local OCI image config %q: %w", reference, err)
	}
	if imageConfig.OS != platform.OS || imageConfig.Architecture != platform.Architecture {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("local image archive %q config platform is %s/%s; VM requires %s", reference, imageConfig.OS, imageConfig.Architecture, platform.String())
	}
	if !sameLocalOCIExecutionConfig(metadata.Config, imageConfig.Config) {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, fmt.Errorf("local image %q changed between Engine inspect and immutable-ID export", reference)
	}
	resolved := resolvedOCIImage{
		Image: image, ImageConfig: imageConfig.Config, RequestedReference: reference,
		SourceManifestDigest: metadata.ID, PlatformManifestDigest: metadata.ID,
		Platform: platform, LocalEngineImageID: metadata.ID, LocalArchivePath: archivePath,
	}
	guestDigest, err := validateGuestAgent(guestAgentPath)
	if err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, err
	}
	prepared, err := prepareRootfsImageFromSource(ctx, cacheDir, formatterPath, guestAgentPath, resolved.cacheIdentity(), guestDigest, cacheMaxBytes, func(rootfsDir string) error {
		return populateRootfsFromOCI(ctx, image, reference, platform, rootfsDir)
	})
	if err != nil {
		return preparedRootfsImage{}, resolvedOCIImage{}, true, err
	}
	keepArchive = true
	return prepared, resolved, true, nil
}

func sameLocalOCIExecutionConfig(engineConfig, archiveConfig v1.Config) bool {
	return slices.Equal(engineConfig.Entrypoint, archiveConfig.Entrypoint) &&
		slices.Equal(engineConfig.Cmd, archiveConfig.Cmd) &&
		slices.Equal(engineConfig.Env, archiveConfig.Env) &&
		engineConfig.User == archiveConfig.User &&
		engineConfig.WorkingDir == archiveConfig.WorkingDir
}

type boundedWriter struct {
	writer  io.Writer
	limit   int64
	written int64
}

var errBoundedWriterLimit = errors.New("bounded writer limit exceeded")

func (w *boundedWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.limit-w.written {
		return 0, errBoundedWriterLimit
	}
	written, err := w.writer.Write(data)
	w.written += int64(written)
	return written, err
}
