/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

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

// Package cmd is the root of our application
package cmd

import (
	"fmt"
	"os"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/spf13/viper"
)

var (
	cfgFile string
	logger  *zap.SugaredLogger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "wallenda",
	Short: "A controller for processing requests for loadbalancers from a specified queue.",
	Long:  `A controller for processing requests for loadbalancers from a specified queue.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.wallenda.yaml)")

	rootCmd.PersistentFlags().Bool("debug", false, "enable debug logging")
	viperBindFlag("logging.debug", rootCmd.PersistentFlags().Lookup("debug"))

	rootCmd.PersistentFlags().Bool("pretty", false, "enable pretty (human readable) logging output")
	viperBindFlag("logging.pretty", rootCmd.PersistentFlags().Lookup("pretty"))

	rootCmd.PersistentFlags().String("nats-url", "nats://127.0.0.1:4222", "NATS server connection url")
	viperBindFlag("nats.url", rootCmd.PersistentFlags().Lookup("nats-url"))

	rootCmd.PersistentFlags().String("nats-nkey", "", "Path to the file containing the NATS nkey keypair")
	viperBindFlag("nats.nkey", rootCmd.PersistentFlags().Lookup("nats-nkey"))

	rootCmd.PersistentFlags().String("nats-subject-prefix", "events.>", "prefix for NATS subjects")
	viperBindFlag("nats.subject-prefix", rootCmd.PersistentFlags().Lookup("nats-subject-prefix"))

	rootCmd.PersistentFlags().String("nats-stream-name", "wallenda", "prefix for NATS subjects")
	viperBindFlag("nats.stream-name", rootCmd.PersistentFlags().Lookup("nats-stream-name"))

	rootCmd.PersistentFlags().String("liveness-port", ":8080", "port to run liveness probe on")
	viperBindFlag("liveness-port", rootCmd.PersistentFlags().Lookup("liveness-port"))

	rootCmd.PersistentFlags().String("readiness-port", ":8081", "port to run readiness probe on")
	viperBindFlag("readiness-port", rootCmd.PersistentFlags().Lookup("readiness-port"))

	rootCmd.PersistentFlags().String("chart-path", "/helm", "path that contains deployment chart")
	viperBindFlag("chart-path", rootCmd.PersistentFlags().Lookup("chart-path"))

	rootCmd.PersistentFlags().String("kube-config-path", "", "path to a valid kubeconfig file")
	viperBindFlag("kube-config-path", rootCmd.PersistentFlags().Lookup("kube-config-path"))

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.AddCommand(processCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".wallenda" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".wallenda")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.SetEnvPrefix("wallenda")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	setupLogging()
}

func setupLogging() {
	cfg := zap.NewProductionConfig()
	if viper.GetBool("logging.pretty") {
		cfg = zap.NewDevelopmentConfig()
	}

	if viper.GetBool("logging.debug") {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	logger = l.Sugar().With("app", "wallenda")
	defer logger.Sync() //nolint:errcheck
}

// viperBindFlag provides a wrapper around the viper bindings that handles error checks
func viperBindFlag(name string, flag *pflag.Flag) {
	err := viper.BindPFlag(name, flag)
	if err != nil {
		panic(err)
	}
}
