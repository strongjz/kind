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

// Package node implements the `add node` command
package node

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
	"sigs.k8s.io/kind/pkg/log"
)

type flagpole struct {
	Name    string
	Role    string
	Image   string
	Retain  bool
}

// NewCommand returns a new cobra.Command for adding a node to an existing cluster
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Use:   "node NODE_NAME",
		Short: "Add a node to an existing cluster",
		Long: `Add a node to an existing cluster.

NODE_NAME must be unique within the cluster.  The node role can be
"worker" (default) or "control-plane".

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
	cmd.Flags().StringVarP(
		&flags.Role,
		"role", "r",
		"worker",
		`node role: "worker" or "control-plane"`,
	)
	cmd.Flags().StringVar(
		&flags.Image,
		"image",
		"",
		"node image to use (defaults to the image used by the cluster)",
	)
	cmd.Flags().BoolVar(
		&flags.Retain,
		"retain",
		false,
		"retain the container for debugging when join fails",
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

	var role config.NodeRole
	switch flags.Role {
	case "worker":
		role = config.WorkerRole
	case "control-plane":
		role = config.ControlPlaneRole
	default:
		return errors.Errorf("invalid role %q: must be \"worker\" or \"control-plane\"", flags.Role)
	}

	nodeConfig := &config.Node{
		Role:  role,
		Image: flags.Image,
	}

	logger.V(0).Infof("Adding %s node %q to cluster %q ...", flags.Role, nodeName, flags.Name)

	node, err := provider.AddNode(flags.Name, nodeName, nodeConfig, flags.Retain)
	if err != nil {
		return errors.Wrap(err, "failed to add node")
	}

	logger.V(0).Infof("Node %s added successfully", node.String())
	return nil
}
