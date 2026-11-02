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
	"sigs.k8s.io/kind/pkg/internal/apis/config/encoding"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
	"sigs.k8s.io/kind/pkg/log"
)

type flagpole struct {
	Name    string
	Role    string
	Image   string
	Retain  bool
	Config  string
	Group   string
}

// NewCommand returns a new cobra.Command for adding a node to an existing cluster
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Use:   "node NODE_NAME",
		Short: "Add a node to an existing cluster",
		Long: `Add a node to an existing cluster.

NODE_NAME must be unique within the cluster.

The node configuration can be specified in three ways (in order of precedence):
  1. Explicit flags (--role, --image)
  2. A named node group from a cluster config file (--config, --group)
  3. Default worker node configuration

Examples:

  # Add a worker node with the default image
  kind add node my-worker --name my-cluster

  # Add a control-plane node
  kind add node my-cp --name my-cluster --role control-plane

  # Add a node using a named group from a config file
  kind add node my-worker --name my-cluster --config cluster.yaml --group workers

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
		"",
		`node role: "worker" (default) or "control-plane"`,
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
	cmd.Flags().StringVar(
		&flags.Config,
		"config",
		"",
		"path to a cluster config file containing nodeGroups",
	)
	cmd.Flags().StringVar(
		&flags.Group,
		"group",
		"",
		"name of a nodeGroup in --config to use as the node template",
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

	nodeConfig, err := resolveNodeConfig(flags)
	if err != nil {
		return err
	}

	logger.V(0).Infof("Adding %s node %q to cluster %q ...", nodeConfig.Role, nodeName, flags.Name)

	node, err := provider.AddNode(flags.Name, nodeName, nodeConfig, flags.Retain)
	if err != nil {
		return errors.Wrap(err, "failed to add node")
	}

	logger.V(0).Infof("Node %s added successfully", node.String())
	return nil
}

// resolveNodeConfig builds a node config from flags and optional cluster config file.
// Priority: explicit --role/--image flags > named --group from --config > defaults.
func resolveNodeConfig(flags *flagpole) (*config.Node, error) {
	nodeConfig := &config.Node{
		Role: config.WorkerRole, // default
	}

	// If a config file and group are provided, load the group template first.
	if flags.Config != "" && flags.Group != "" {
		clusterCfg, err := encoding.Load(flags.Config)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load config file %q", flags.Config)
		}
		groupFound := false
		for _, g := range clusterCfg.NodeGroups {
			if g.Name == flags.Group {
				nodeConfig = g.Template.DeepCopy()
				groupFound = true
				break
			}
		}
		if !groupFound {
			return nil, errors.Errorf("node group %q not found in config file %q", flags.Group, flags.Config)
		}
	} else if flags.Config != "" && flags.Group == "" {
		return nil, errors.New("--config requires --group to specify which nodeGroup to use")
	} else if flags.Group != "" && flags.Config == "" {
		return nil, errors.New("--group requires --config to specify the cluster config file")
	}

	// Explicit flags override the group template.
	if flags.Role != "" {
		switch flags.Role {
		case "worker":
			nodeConfig.Role = config.WorkerRole
		case "control-plane":
			nodeConfig.Role = config.ControlPlaneRole
		default:
			return nil, errors.Errorf("invalid role %q: must be \"worker\" or \"control-plane\"", flags.Role)
		}
	}
	if flags.Image != "" {
		nodeConfig.Image = flags.Image
	}

	return nodeConfig, nil
}
