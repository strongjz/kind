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
	"fmt"
	"strings"

	"sigs.k8s.io/kind/pkg/cluster/internal/providers/common"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// EtcdMember holds metadata about a single etcd cluster member.
type EtcdMember struct {
	ID        string
	Name      string
	PeerURL   string
	ClientURL string
	IsLearner bool
}

// RemoveEtcdMember removes the named control-plane node from the etcd cluster.
// allNodes must include all current cluster nodes (the node being removed may still be present).
func RemoveEtcdMember(logger log.Logger, allNodes []nodes.Node, nodeToRemove string) error {
	logger.V(1).Infof("Removing etcd member for node %s", nodeToRemove)

	// Pick a healthy control-plane node (not the one being removed) to run commands.
	var healthyNodes []nodes.Node
	for _, n := range allNodes {
		if n.String() != nodeToRemove {
			healthyNodes = append(healthyNodes, n)
		}
	}

	controlPlaneNode, err := nodeutils.BootstrapControlPlaneNode(healthyNodes)
	if err != nil {
		return errors.Wrap(err, "failed to find healthy control plane node for etcd operations")
	}

	members, err := listEtcdMembers(logger, controlPlaneNode)
	if err != nil {
		return errors.Wrap(err, "failed to list etcd members")
	}

	logger.V(2).Infof("etcd has %d members before removal", len(members))

	var target *EtcdMember
	for i := range members {
		if members[i].Name == nodeToRemove {
			target = &members[i]
			break
		}
	}
	if target == nil {
		logger.Warnf("Node %s not found in etcd member list; skipping etcd removal", nodeToRemove)
		return nil
	}
	if len(members) <= 1 {
		return errors.New("cannot remove the last etcd member")
	}

	logger.V(0).Infof("Removing etcd member %s (ID: %s)", target.Name, target.ID)
	lines, err := etcdctl(controlPlaneNode, "member", "remove", target.ID)
	if err != nil {
		logger.V(3).Info(strings.Join(lines, "\n"))
		return errors.Wrapf(err, "failed to remove etcd member %s", target.ID)
	}

	logger.V(0).Infof("Removed etcd member %s; cluster now has %d members", nodeToRemove, len(members)-1)
	return nil
}

// etcdctl runs etcdctl inside the etcd container on the given control-plane node.
func etcdctl(controlPlaneNode nodes.Node, args ...string) ([]string, error) {
	// Find the running etcd container ID.
	containerLines, err := exec.OutputLines(controlPlaneNode.Command("crictl", "ps", "--name", "etcd", "-q"))
	if err != nil || len(containerLines) == 0 {
		return nil, errors.New("no running etcd container found")
	}
	etcdID := strings.TrimSpace(containerLines[0])

	base := []string{
		"crictl", "exec", etcdID, "etcdctl",
		fmt.Sprintf("--endpoints=https://127.0.0.1:%d", common.EtcdClientPort),
		"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
		"--cert=/etc/kubernetes/pki/etcd/server.crt",
		"--key=/etc/kubernetes/pki/etcd/server.key",
	}
	base = append(base, args...)
	return exec.CombinedOutputLines(controlPlaneNode.Command(base[0], base[1:]...))
}

func listEtcdMembers(logger log.Logger, controlPlaneNode nodes.Node) ([]EtcdMember, error) {
	lines, err := etcdctl(controlPlaneNode, "member", "list")
	if err != nil {
		return nil, errors.Wrap(err, "etcdctl member list failed")
	}
	var members []EtcdMember
	for _, line := range lines {
		m, err := parseEtcdMemberLine(line)
		if err != nil {
			logger.V(2).Infof("Skipping unparseable etcd member line: %q", line)
			continue
		}
		members = append(members, m)
	}
	return members, nil
}

// parseEtcdMemberLine parses a line from `etcdctl member list`.
// Format: <id>, <status>, <name>, <peerURL>, <clientURL>, <isLearner>
func parseEtcdMemberLine(line string) (EtcdMember, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return EtcdMember{}, errors.New("empty line")
	}
	parts := strings.SplitN(line, ",", 6)
	if len(parts) < 6 {
		return EtcdMember{}, errors.Errorf("unexpected etcd member line format: %q", line)
	}
	return EtcdMember{
		ID:        strings.TrimSpace(parts[0]),
		Name:      strings.TrimSpace(parts[2]),
		PeerURL:   strings.TrimSpace(parts[3]),
		ClientURL: strings.TrimSpace(parts[4]),
		IsLearner: strings.TrimSpace(parts[5]) == "true",
	}, nil
}
