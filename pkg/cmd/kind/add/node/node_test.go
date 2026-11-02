/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package node

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

func TestResolveNodeConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		flags      flagpole
		wantRole   config.NodeRole
		wantImage  string
		wantErrMsg string
	}{
		{
			name:     "defaults to worker when no flags set",
			flags:    flagpole{},
			wantRole: config.WorkerRole,
		},
		{
			name:     "explicit worker role",
			flags:    flagpole{Role: "worker"},
			wantRole: config.WorkerRole,
		},
		{
			name:     "explicit control-plane role",
			flags:    flagpole{Role: "control-plane"},
			wantRole: config.ControlPlaneRole,
		},
		{
			name:      "explicit image overrides default",
			flags:     flagpole{Image: "kindest/node:v1.29.0"},
			wantRole:  config.WorkerRole,
			wantImage: "kindest/node:v1.29.0",
		},
		{
			name:       "invalid role",
			flags:      flagpole{Role: "invalid"},
			wantErrMsg: "invalid role",
		},
		{
			name:       "--config without --group returns error",
			flags:      flagpole{Config: "some.yaml"},
			wantErrMsg: "--config requires --group",
		},
		{
			name:       "--group without --config returns error",
			flags:      flagpole{Group: "workers"},
			wantErrMsg: "--group requires --config",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveNodeConfig(&tc.flags)
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrMsg)
				}
				if errStr := err.Error(); len(errStr) == 0 {
					t.Fatalf("expected non-empty error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Role != tc.wantRole {
				t.Errorf("Role: got %q, want %q", got.Role, tc.wantRole)
			}
			if got.Image != tc.wantImage {
				t.Errorf("Image: got %q, want %q", got.Image, tc.wantImage)
			}
		})
	}
}

func TestResolveNodeConfigFromGroup(t *testing.T) {
	t.Parallel()

	// Write a temporary cluster config with nodeGroups
	cfgYAML := `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodeGroups:
- name: workers
  template:
    role: worker
    image: kindest/node:v1.29.0
    labels:
      node-group: workers
- name: control-planes
  template:
    role: control-plane
    image: kindest/node:v1.29.0
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cases := []struct {
		name       string
		flags      flagpole
		wantRole   config.NodeRole
		wantImage  string
		wantErrMsg string
	}{
		{
			name:      "worker group from config",
			flags:     flagpole{Config: cfgPath, Group: "workers"},
			wantRole:  config.WorkerRole,
			wantImage: "kindest/node:v1.29.0",
		},
		{
			name:      "control-plane group from config",
			flags:     flagpole{Config: cfgPath, Group: "control-planes"},
			wantRole:  config.ControlPlaneRole,
			wantImage: "kindest/node:v1.29.0",
		},
		{
			name:      "explicit --image overrides group image",
			flags:     flagpole{Config: cfgPath, Group: "workers", Image: "kindest/node:v1.30.0"},
			wantRole:  config.WorkerRole,
			wantImage: "kindest/node:v1.30.0",
		},
		{
			name:      "explicit --role overrides group role",
			flags:     flagpole{Config: cfgPath, Group: "workers", Role: "control-plane"},
			wantRole:  config.ControlPlaneRole,
			wantImage: "kindest/node:v1.29.0",
		},
		{
			name:       "non-existent group returns error",
			flags:      flagpole{Config: cfgPath, Group: "missing"},
			wantErrMsg: "not found",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveNodeConfig(&tc.flags)
			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Role != tc.wantRole {
				t.Errorf("Role: got %q, want %q", got.Role, tc.wantRole)
			}
			if got.Image != tc.wantImage {
				t.Errorf("Image: got %q, want %q", got.Image, tc.wantImage)
			}
		})
	}
}
