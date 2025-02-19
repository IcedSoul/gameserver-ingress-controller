/*
Copyright © 2021 OCTOPS.IO

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
package cmd

import (
	"context"
	"fmt"
	"github.com/Octops/gameserver-ingress-controller/internal/runtime"
	"github.com/Octops/gameserver-ingress-controller/pkg/app"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var (
	cfgFile                string
	masterURL              string
	kubeconfig             string
	syncPeriod             string
	webhookPort            int
	healthProbeBindAddress string
	metricsBindAddress     string
	verbose                bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "octops-controller",
	Short: "Automatic Ingress configuration for Game Servers",
	Long: `The octops-controller watches for game servers managed by Agones and creates Ingress resources that 
makes the traffic to be routed to the game server using an Ingress Controller.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := runtime.SetupSignal(context.Background())
		defer stop()

		logger := runtime.NewLogger(verbose)
		app.StartController(ctx, logger, app.Config{
			Kubeconfig:             kubeconfig,
			SyncPeriod:             syncPeriod,
			Port:                   webhookPort,
			HealthProbeBindAddress: healthProbeBindAddress,
			MetricsBindAddress:     metricsBindAddress,
			Verbose:                verbose,
		})
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gameserver-ingress-controller.yaml)")

	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Set KUBECONFIG")
	rootCmd.Flags().StringVar(&masterURL, "master", "", "The addr of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	rootCmd.Flags().StringVar(&syncPeriod, "sync-period", "15s", "Set the minimum frequency at which watched resources are reconciled")
	rootCmd.Flags().StringVar(&healthProbeBindAddress, "health-probe-addrs", ":30235", "TCP address that the controller should bind to for serving health probes")
	rootCmd.Flags().StringVar(&metricsBindAddress, "metrics-addrs", ":9090", "TCP address that the controller should bind to for serving prometheus metrics")
	rootCmd.Flags().IntVar(&webhookPort, "webhook-port", 30234, "Port used by the controller for webhooks")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Produce verbose log")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".gameserver-ingress-controller" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".gameserver-ingress-controller")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
