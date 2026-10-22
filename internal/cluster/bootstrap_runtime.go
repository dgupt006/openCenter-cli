package cluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	corePaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
)

type bootstrapRuntimePaths struct {
	StatePath       string
	LegacyStatePath string
	LogPath         string
}

type bootstrapLogWriterKey struct{}

func resolveBootstrapRuntimePaths(cfg *v2.Config, logPath string, startedAt time.Time) (*bootstrapRuntimePaths, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	clusterName := sanitizeRuntimeSegment(cfg.ClusterName())
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be set")
	}
	organization := sanitizeRuntimeSegment(cfg.Organization())
	if organization == "" {
		organization = "opencenter"
	}

	stateDir := config.GetStateDir()
	statePath, err := resolveBootstrapPath(filepath.Join(stateDir, "bootstrap", organization, clusterName, "state.json"))
	if err != nil {
		return nil, fmt.Errorf("resolve bootstrap state path: %w", err)
	}

	legacyStatePath := ""
	if legacyClusterDir, err := infrastructureClusterDir(cfg); err == nil {
		legacyStatePath = filepath.Join(legacyClusterDir, "logs", "bootstrap-state.json")
	}

	resolvedLogPath := strings.TrimSpace(logPath)
	if resolvedLogPath == "" {
		resolvedLogPath = filepath.Join(stateDir, "logs", "bootstrap", organization, clusterName, bootstrapLogFileName(startedAt))
	}
	resolvedLogPath, err = resolveBootstrapPath(resolvedLogPath)
	if err != nil {
		return nil, fmt.Errorf("resolve bootstrap log path: %w", err)
	}

	return &bootstrapRuntimePaths{
		StatePath:       statePath,
		LegacyStatePath: legacyStatePath,
		LogPath:         resolvedLogPath,
	}, nil
}

func bootstrapLogFileName(startedAt time.Time) string {
	return "bootstrap-" + startedAt.UTC().Format("20060102T150405Z") + ".log"
}

// ResolveDestroyLogPath resolves the log path for cluster destroy operations.
func ResolveDestroyLogPath(cfg *v2.Config, logPath string, startedAt time.Time) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("configuration is nil")
	}

	clusterName := sanitizeRuntimeSegment(cfg.ClusterName())
	if clusterName == "" {
		return "", fmt.Errorf("cluster name must be set")
	}
	organization := sanitizeRuntimeSegment(cfg.Organization())
	if organization == "" {
		organization = "opencenter"
	}

	stateDir := config.GetStateDir()
	resolvedLogPath := strings.TrimSpace(logPath)
	if resolvedLogPath == "" {
		resolvedLogPath = filepath.Join(stateDir, "logs", "destroy", organization, clusterName, destroyLogFileName(startedAt))
	}
	return resolveBootstrapPath(resolvedLogPath)
}

func destroyLogFileName(startedAt time.Time) string {
	return "destroy-" + startedAt.UTC().Format("20060102T150405Z") + ".log"
}

func resolveBootstrapPath(path string) (string, error) {
	expanded := corePaths.ExpandPath(strings.TrimSpace(path))
	if expanded == "" {
		return "", nil
	}
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded), nil
	}
	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func sanitizeRuntimeSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, string(os.PathSeparator), "-")
	return value
}

// OpenActionLogFile creates the directory if needed and opens a log file for writing.
func OpenActionLogFile(path string) (*os.File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	return file, nil
}

func openBootstrapLogFile(path string) (*os.File, error) {
	return OpenActionLogFile(path)
}

func withBootstrapLogWriter(ctx context.Context, writer io.Writer) context.Context {
	if writer == nil {
		return ctx
	}
	ctx = logging.WithActionLogWriter(ctx, writer)
	return context.WithValue(ctx, bootstrapLogWriterKey{}, writer)
}

func bootstrapLogWriter(ctx context.Context) io.Writer {
	if ctx == nil {
		return nil
	}
	if writer, ok := ctx.Value(bootstrapLogWriterKey{}).(io.Writer); ok && writer != nil {
		return writer
	}
	return logging.ActionLogWriter(ctx)
}

func logBootstrapMessage(ctx context.Context, format string, args ...interface{}) {
	logging.LogActionMessage(ctx, format, args...)
}
