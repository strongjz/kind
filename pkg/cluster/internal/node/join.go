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

// Package node implements node management operations for existing clusters.
package node

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/log"
)

// JoinNodeToCluster joins a new node to an existing cluster using kubeadm.
func JoinNodeToCluster(logger log.Logger, provider providers.Provider, cluster string, node nodes.Node, nodeRole config.NodeRole) error {
	allNodes, err := provider.ListNodes(cluster)
	if err != nil {
		return errors.Wrap(err, "failed to list cluster nodes")
	}

	// Build the set of existing nodes, excluding the node being joined.
	var existingNodes []nodes.Node
	for _, n := range allNodes {
		if n.String() != node.String() {
			existingNodes = append(existingNodes, n)
		}
	}

	controlPlaneNode, err := nodeutils.BootstrapControlPlaneNode(existingNodes)
	if err != nil {
		return errors.Wrap(err, "failed to find bootstrap control plane node")
	}

	joinCommand, err := generateJoinCommand(logger, controlPlaneNode, nodeRole)
	if err != nil {
		return errors.Wrap(err, "failed to generate join command")
	}

	if err := runJoinCommand(logger, node, joinCommand); err != nil {
		return errors.Wrap(err, "failed to run join command")
	}

	if err := waitForNodeReady(logger, node, 2*time.Minute); err != nil {
		return errors.Wrap(err, "node failed to become ready")
	}

	if err := waitForNodeVisibleInAPI(logger, controlPlaneNode, node.String(), 2*time.Minute); err != nil {
		return errors.Wrap(err, "node failed to become visible in Kubernetes API")
	}

	return nil
}

// generateJoinCommand creates a kubeadm join command from an existing control plane node.
// For control-plane roles it also uploads certs and includes --control-plane.
func generateJoinCommand(logger log.Logger, controlPlaneNode nodes.Node, nodeRole config.NodeRole) ([]string, error) {
	if nodeRole == config.ControlPlaneRole {
		certKey, err := generateAndUploadCerts(logger, controlPlaneNode)
		if err != nil {
			return nil, errors.Wrap(err, "failed to upload certificates")
		}
		cmd := controlPlaneNode.Command("kubeadm", "token", "create",
			"--print-join-command",
			"--certificate-key", certKey,
		)
		lines, err := exec.CombinedOutputLines(cmd)
		if err != nil {
			logger.V(3).Info(strings.Join(lines, "\n"))
			return nil, errors.Wrap(err, "failed to create join token for control-plane")
		}
		return strings.Fields(strings.Join(lines, " ")), nil
	}

	cmd := controlPlaneNode.Command("kubeadm", "token", "create", "--print-join-command")
	lines, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		logger.V(3).Info(strings.Join(lines, "\n"))
		return nil, errors.Wrap(err, "failed to create join token for worker")
	}
	return strings.Fields(strings.Join(lines, " ")), nil
}

// generateAndUploadCerts generates a random certificate key, uploads the cluster
// certificates to the Kubernetes API, and returns the key.
func generateAndUploadCerts(logger log.Logger, controlPlaneNode nodes.Node) (string, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", errors.Wrap(err, "failed to generate random bytes for certificate key")
	}
	certKey := hex.EncodeToString(keyBytes)

	cmd := controlPlaneNode.Command("kubeadm", "init", "phase",
		"upload-certs",
		"--certificate-key", certKey,
		"--upload-certs",
	)
	lines, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		logger.V(3).Info(strings.Join(lines, "\n"))
		return "", errors.Wrap(err, "failed to upload certificates")
	}

	logger.V(2).Infof("Uploaded certificates with key prefix %s...", certKey[:8])
	return certKey, nil
}

// runJoinCommand executes the kubeadm join command on the target node.
func runJoinCommand(logger log.Logger, node nodes.Node, joinArgs []string) error {
	logger.V(0).Infof("Running kubeadm join on node %s", node.String())
	cmd := node.Command(joinArgs[0], joinArgs[1:]...)
	lines, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		logger.V(3).Info(strings.Join(lines, "\n"))
		return errors.Wrap(err, "kubeadm join failed")
	}
	return nil
}

// waitForNodeReady polls until kubelet on the node reports active.
func waitForNodeReady(logger log.Logger, node nodes.Node, timeout time.Duration) error {
	logger.V(0).Infof("Waiting for kubelet on node %s...", node.String())
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := node.Command("systemctl", "is-active", "kubelet").Run(); err == nil {
			logger.V(0).Infof("kubelet is active on node %s", node.String())
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return errors.Errorf("node %s did not become ready within %v", node.String(), timeout)
}

// waitForNodeVisibleInAPI polls kubectl on the control plane until the node appears.
func waitForNodeVisibleInAPI(logger log.Logger, controlPlaneNode nodes.Node, nodeName string, timeout time.Duration) error {
	logger.V(0).Infof("Waiting for node %s in Kubernetes API...", nodeName)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := controlPlaneNode.Command("kubectl", "get", "nodes", nodeName, "--no-headers").Run(); err == nil {
			logger.V(0).Infof("Node %s is visible in Kubernetes API", nodeName)
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return errors.Errorf("node %s did not become visible in Kubernetes API within %v", nodeName, timeout)
}
