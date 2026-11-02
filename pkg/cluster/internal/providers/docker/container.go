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

package docker

import (
	"path/filepath"

	"sigs.k8s.io/kind/pkg/cluster/internal/providers/common"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/fs"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// prepareNodeForAddition deep-copies the node config, applies defaults, and
// resolves any relative host-path mounts to absolute paths.
func prepareNodeForAddition(nodeConfig *config.Node) (*config.Node, error) {
	node := nodeConfig.DeepCopy()
	config.SetDefaultsNode(node)
	for i := range node.ExtraMounts {
		hp := node.ExtraMounts[i].HostPath
		if !fs.IsAbs(hp) {
			abs, err := filepath.Abs(hp)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to resolve absolute path for hostPath: %q", hp)
			}
			node.ExtraMounts[i].HostPath = abs
		}
	}
	return node, nil
}

// buildRunArgs assembles docker run arguments for a new node being added to
// an existing cluster.  For control-plane nodes it binds a random host port to
// the API server container port; workers need no extra port mapping.
func buildRunArgs(cluster, networkName string, allNodeNames []string, node *config.Node, nodeName string, ipFamily config.ClusterIPFamily) ([]string, error) {
	// Re-use the same clusterConfig-based commonArgs as initial provisioning.
	// We synthesize a minimal cluster config with the networking IP family so
	// that proxy environment variables and IPv6 sysctls are handled correctly.
	clusterCfg := &config.Cluster{
		Name:  cluster,
		Nodes: []config.Node{*node},
		Networking: config.Networking{
			IPFamily: ipFamily,
		},
	}

	genericArgs, err := commonArgs(cluster, clusterCfg, networkName, allNodeNames)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate common container args")
	}

	switch node.Role {
	case config.ControlPlaneRole:
		// Assign a random host port for the API server.
		node.ExtraPortMappings = append(node.ExtraPortMappings, config.PortMapping{
			ListenAddress: "127.0.0.1",
			HostPort:      0, // random
			ContainerPort: common.APIServerInternalPort,
		})
	case config.WorkerRole:
		// no additional port mappings required
	default:
		return nil, errors.Errorf("unsupported node role: %q", node.Role)
	}

	return runArgsForNode(node, ipFamily, nodeName, genericArgs)
}
