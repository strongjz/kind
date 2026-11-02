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

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/apis/config/encoding"
)

func TestNodeGroupRoundTrip(t *testing.T) {
	t.Parallel()

	yaml := `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodeGroups:
- name: workers
  template:
    role: worker
    image: kindest/node:v1.32.0
    labels:
      tier: backend
- name: infra
  template:
    role: worker
    image: kindest/node:v1.32.0
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := encoding.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.NodeGroups) != 2 {
		t.Fatalf("expected 2 node groups, got %d", len(cfg.NodeGroups))
	}

	workers := cfg.NodeGroups[0]
	if workers.Name != "workers" {
		t.Errorf("NodeGroups[0].Name: got %q, want %q", workers.Name, "workers")
	}
	if workers.Template.Role != config.WorkerRole {
		t.Errorf("NodeGroups[0].Template.Role: got %q, want %q", workers.Template.Role, config.WorkerRole)
	}
	if workers.Template.Image != "kindest/node:v1.32.0" {
		t.Errorf("NodeGroups[0].Template.Image: got %q", workers.Template.Image)
	}
	if workers.Template.Labels["tier"] != "backend" {
		t.Errorf("NodeGroups[0].Template.Labels[tier]: got %q, want %q",
			workers.Template.Labels["tier"], "backend")
	}

	infra := cfg.NodeGroups[1]
	if infra.Name != "infra" {
		t.Errorf("NodeGroups[1].Name: got %q, want %q", infra.Name, "infra")
	}
}

func TestNodeGroupDeepCopy(t *testing.T) {
	t.Parallel()

	original := config.NodeGroup{
		Name: "workers",
		Template: config.Node{
			Role:  config.WorkerRole,
			Image: "kindest/node:v1.32.0",
			Labels: map[string]string{
				"group": "workers",
			},
		},
	}

	copy := original.DeepCopy()
	if copy == nil {
		t.Fatal("DeepCopy returned nil")
	}
	if copy.Name != original.Name {
		t.Errorf("Name: got %q, want %q", copy.Name, original.Name)
	}

	// Mutating the copy should not affect the original.
	copy.Template.Labels["group"] = "mutated"
	if original.Template.Labels["group"] != "workers" {
		t.Error("DeepCopy did not deep-copy labels map")
	}
}

func TestNodeGroupNotPresentByDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.Cluster{}
	config.SetDefaultsCluster(cfg)

	if len(cfg.NodeGroups) != 0 {
		t.Errorf("expected no node groups by default, got %d", len(cfg.NodeGroups))
	}
}
