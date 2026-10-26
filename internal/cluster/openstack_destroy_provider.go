// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

type openstackDestroyProvider struct {
	runner lifecycleCommandRunner
	output io.Writer
	// cinderFactory creates the Cinder service client. Override in tests to
	// avoid live OpenStack calls.
	cinderFactory func(*credentials.OpenStackCredentials) (cinderVolumeService, error)
}

func newOpenStackDestroyProvider(runner lifecycleCommandRunner, output io.Writer) lifecycleDestroyProvider {
	return &openstackDestroyProvider{
		runner:        runner,
		output:        output,
		cinderFactory: newGophercloudCinderService,
	}
}

func (p *openstackDestroyProvider) BuildSteps(cfg *v2.Config, opts *DestroyInfraOptions) ([]destroyStep, error) {
	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return nil, err
	}

	if !cfg.OpenTofu.Enabled {
		return nil, fmt.Errorf("opentofu must be enabled for openstack destroy")
	}

	// An absent infrastructure directory means there is nothing left to run
	// OpenTofu against: the infrastructure was already destroyed, or the
	// GitOps working directory (gitops.repository.local_dir) no longer exists
	// (e.g. a checkout under a deleted temp directory). Treat it as an
	// idempotent no-op so `cluster destroy --remove-files` can still complete
	// local cleanup instead of failing permanently on a vanished directory.
	infraAlreadyGone := false
	if _, err := os.Stat(clusterDir); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("checking cluster infrastructure directory: %w", err)
		}
		infraAlreadyGone = true
		p.logf("Cluster infrastructure directory not found: %s\n", clusterDir)
		p.logf("Nothing to destroy via OpenTofu; infrastructure may already be destroyed.\n")
	}

	var (
		env          map[string]string
		openTofuPath string
		destroyArgs  []string
		creds        *credentials.OpenStackCredentials
	)

	// The tofu binary and OpenStack credentials are only needed when there is
	// an infrastructure directory to destroy. Skip them when the directory is
	// gone so a broken credential section cannot block local file cleanup.
	if !infraAlreadyGone {
		extracted, err := credentials.NewExtractor(*cfg).ExtractOpenStack()
		if err != nil {
			return nil, fmt.Errorf("extract openstack credentials: %w", err)
		}

		env = buildDestroyEnvironment()
		mergeBootstrapEnvironment(env, extracted.ToEnvMap())
		creds = extracted

		openTofuPath, err = resolveTofuBinary(cfg.OpenTofu.Path)
		if err != nil {
			return nil, err
		}

		destroyArgs = []string{"destroy"}
		if opts != nil && opts.AutoApprove {
			destroyArgs = append(destroyArgs, "-auto-approve")
		}
	}

	// CSI volume cleanup is only applicable to the OpenStack provider. The
	// VMware provider shares this same struct but does not use Cinder.
	isOpenStack := strings.EqualFold(cfg.Provider(), "openstack")

	// capturedHandles is shared between the capture step and the cleanup step
	// via closure. It is populated before opentofu-destroy runs.
	var capturedHandles []string

	var steps []destroyStep

	if isOpenStack {
		// Derive kubeconfig path from the same base as clusterDir to ensure consistency.
		// Both capture and tofu use the same infrastructure root so they stay in sync
		// across layout changes (flat vs org-scoped).
		kubeconfigPath := filepath.Join(clusterDir, "kubeconfig.yaml")
		steps = append(steps, destroyStep{
			ID:          "capture-csi-volumes",
			Description: "Capture CSI-provisioned Cinder volume handles from cluster",
			Run: func(ctx context.Context) error {
				handles, err := collectCSIVolumeHandles(ctx, kubeconfigPath, p.runner)
				if err != nil {
					// Non-fatal: the cluster may already be unreachable.
					p.logf("Warning: could not capture CSI volume handles: %v\n", err)
					p.logf("Orphaned Cinder volumes may need manual cleanup after destroy.\n")
					return nil
				}
				capturedHandles = handles
				p.logf("Captured %d CSI-provisioned Cinder volume handle(s).\n", len(handles))
				return nil
			},
		})
	}

	if !infraAlreadyGone {
		steps = append(steps,
			destroyStep{
				ID:          "opentofu-init",
				Description: "Initialize OpenTofu",
				Run: func(ctx context.Context) error {
					_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, "init")
					return err
				},
			},
			destroyStep{
				ID:          "opentofu-destroy",
				Description: "Destroy OpenTofu infrastructure",
				Run: func(ctx context.Context) error {
					_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, destroyArgs...)
					return err
				},
			},
		)
	}

	if isOpenStack {
		deleteVolumes := opts != nil && opts.DeleteVolumes
		steps = append(steps, destroyStep{
			ID:          "cleanup-csi-volumes",
			Description: "Delete or report orphaned CSI-provisioned Cinder volumes",
			Run: func(ctx context.Context) error {
				if len(capturedHandles) == 0 {
					p.logf("No CSI-provisioned Cinder volumes to clean up.\n")
					return nil
				}
				svc, err := p.cinderFactory(creds)
				if err != nil {
					// Non-fatal: infra is already destroyed; warn and list handles.
					p.logf("Warning: could not create Cinder client for volume cleanup: %v\n", err)
					p.logf("The following %d volume(s) may need manual cleanup:\n", len(capturedHandles))
					for _, h := range capturedHandles {
						p.logf("  - %s\n", h)
					}
					return nil
				}
				if err := cleanupCSIVolumes(ctx, capturedHandles, svc, deleteVolumes, p.output); err != nil {
					// In delete-mode: surface error so caller can signal non-zero exit.
					// In report-only: log warning and continue (destruction already succeeded).
					if deleteVolumes {
						p.logf("ERROR: CSI volume cleanup failed: %v\n", err)
						p.logf("Verify that all volumes were deleted. Some may still be orphaned.\n")
						// Return the error; the provider will track it for the caller.
						return fmt.Errorf("volume cleanup: %w", err)
					}
					p.logf("Warning: CSI volume cleanup encountered errors: %v\n", err)
					p.logf("Some volumes may need manual cleanup.\n")
				}
				return nil
			},
		})
	}

	return steps, nil
}

func (p *openstackDestroyProvider) logf(format string, args ...any) {
	if p.output != nil {
		fmt.Fprintf(p.output, format, args...)
	}
}

// buildDestroyEnvironment creates the environment variables for destroy operations.
func buildDestroyEnvironment() map[string]string {
	env := make(map[string]string)

	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}
