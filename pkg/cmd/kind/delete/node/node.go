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

// Package node implements the `delete node` command
package node

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/constants"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
	"sigs.k8s.io/kind/pkg/log"
)

type flagpole struct {
	Name string
}

// NewCommand returns a new cobra.Command for removing a node from a cluster
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Use:   "node NODE_NAME",
		Short: "Remove a node from an existing cluster",
		Long: `Remove a node from an existing cluster.

The node is gracefully drained before being removed from the Kubernetes
control plane.  For control-plane nodes the etcd member is also removed.
The last control-plane node cannot be deleted.

NOTE: only the Docker provider is currently supported.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(logger, flags, args[0])
		},
	}
	cmd.Flags().StringVarP(
		&flags.Name,
		"name", "n",
		cluster.DefaultName,
		"cluster name",
	)
	return cmd
}

func runE(logger log.Logger, flags *flagpole, nodeName string) error {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	clusters, err := provider.List()
	if err != nil {
		return errors.Wrap(err, "failed to list clusters")
	}
	found := false
	for _, c := range clusters {
		if c == flags.Name {
			found = true
			break
		}
	}
	if !found {
		return errors.Errorf("cluster %q does not exist", flags.Name)
	}

	clusterNodes, err := provider.ListNodes(flags.Name)
	if err != nil {
		return errors.Wrap(err, "failed to list cluster nodes")
	}

	// Verify the node exists in the cluster.
	nodeExists := false
	for _, n := range clusterNodes {
		if n.String() == nodeName {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		return errors.Errorf("node %q does not exist in cluster %q", nodeName, flags.Name)
	}

	// Refuse to delete the last control-plane node.
	for _, n := range clusterNodes {
		if n.String() != nodeName {
			continue
		}
		role, err := n.Role()
		if err != nil {
			return errors.Wrapf(err, "failed to determine role of node %q", nodeName)
		}
		if role == constants.ControlPlaneNodeRoleValue {
			cpNodes, err := nodeutils.ControlPlaneNodes(clusterNodes)
			if err != nil {
				return errors.Wrap(err, "failed to list control-plane nodes")
			}
			if len(cpNodes) <= 1 {
				return errors.New("cannot delete the last control-plane node")
			}
		}
		break
	}

	logger.V(0).Infof("Removing node %q from cluster %q ...", nodeName, flags.Name)

	if err := provider.RemoveNode(flags.Name, nodeName); err != nil {
		return errors.Wrap(err, "failed to remove node")
	}

	logger.V(0).Infof("Node %q removed successfully", nodeName)
	return nil
}
