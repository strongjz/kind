/*
Copyright 2018 The Kubernetes Authors.

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

// Package cluster implements the `create cluster` command
package cluster

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/runtime"
)

type flagpole struct {
	Name       string
	Config     string
	ImageName  string
	Retain     bool
	Wait       time.Duration
	Kubeconfig string
}

// NewCommand returns a new cobra.Command for cluster creation
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "cluster --name [cluster context name]",
		Short: "start one of cluster",
		Long:  "Starts one of local Kubernetes cluster (cluster) in --name",
		RunE: func(cmd *cobra.Command, args []string) error {
			return startCluster(logger, streams, flags)
		},
	}
	cmd.Flags().StringVar(&flags.Name, "name", "", "cluster context name")

	return cmd
}

func startCluster(logger log.Logger, streams cmd.IOStreams, flags *flagpole) error {

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	clusters, err := provider.List()
	if err != nil {
		return err
	}
	if len(clusters) == 0 {
		logger.V(0).Info("No kind clusters found.")
		return nil
	}

	//does the named cluster exist
	sort.Strings(clusters)
	i := sort.SearchStrings(clusters,flags.Name)

	if i < len(clusters) && clusters[i] == flags.Name {
		fmt.Fprintln(streams.Out, "Cluster "+clusters[i]+" exist")
		return nil
	}

	//if not list out all all the options
	fmt.Fprintln(streams.Out, "Cluster "+flags.Name+" does not exist")

	fmt.Fprintln(streams.Out, "List of Available Clusters to start")

	for _, cluster := range clusters {
		fmt.Fprintln(streams.Out, cluster)
	}
	return nil
}
