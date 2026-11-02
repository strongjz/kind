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
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// DrainAndRemoveNode cordon-drains a node and removes it from the Kubernetes cluster.
// It does not delete the underlying container; that is the caller's responsibility.
func DrainAndRemoveNode(logger log.Logger, provider providers.Provider, cluster, nodeName string) error {
	allNodes, err := provider.ListNodes(cluster)
	if err != nil {
		return errors.Wrap(err, "failed to list cluster nodes")
	}

	controlPlaneNode, err := nodeutils.BootstrapControlPlaneNode(allNodes)
	if err != nil {
		return errors.Wrap(err, "failed to find bootstrap control plane node")
	}

	logger.V(0).Infof("Cordoning node %s...", nodeName)
	if err := controlPlaneNode.Command("kubectl", "cordon", nodeName).Run(); err != nil {
		logger.Warnf("Failed to cordon node %s: %v (continuing)", nodeName, err)
	}

	logger.V(0).Infof("Draining node %s...", nodeName)
	if err := drainNode(logger, controlPlaneNode, nodeName); err != nil {
		logger.Warnf("Failed to drain node %s: %v (continuing)", nodeName, err)
	}

	logger.V(0).Infof("Deleting node %s from cluster...", nodeName)
	if err := controlPlaneNode.Command("kubectl", "delete", "node", nodeName).Run(); err != nil {
		logger.Warnf("Failed to delete node %s from cluster: %v (continuing)", nodeName, err)
	}

	return nil
}

func drainNode(logger log.Logger, controlPlaneNode nodes.Node, nodeName string) error {
	args := []string{
		"drain", nodeName,
		"--ignore-daemonsets",
		"--delete-emptydir-data",
		"--force",
		"--timeout=60s",
	}
	cmd := controlPlaneNode.Command("kubectl", args...)
	lines, err := exec.CombinedOutputLines(cmd)
	logger.V(3).Info(strings.Join(lines, "\n"))
	if err != nil {
		return errors.Wrap(err, "failed to drain node")
	}
	return nil
}

// ResetNode runs kubeadm reset on a node to clean up Kubernetes state.
func ResetNode(logger log.Logger, node nodes.Node) error {
	logger.V(0).Infof("Resetting Kubernetes components on node %s...", node.String())
	cmd := node.Command("kubeadm", "reset", "--force")
	lines, err := exec.CombinedOutputLines(cmd)
	logger.V(3).Info(strings.Join(lines, "\n"))
	if err != nil {
		logger.Warnf("Failed to reset node %s: %v (continuing)", node.String(), err)
	}
	return nil
}
