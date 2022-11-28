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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/infratographer/wallenda/internal/srv"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// processCmd represents the base command when called without any subcommands
var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Begin processing requests from queues.",
	Long:  `Begin processing requests from message queues to create LBs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return process(cmd.Context())
	},
}

func process(ctx context.Context) error {
	kubeconfig := viper.GetString("kube-config-path")
	client, err := newKubeAuth(kubeconfig)

	if err != nil {
		logger.Fatalw("failed to create Kubernetes client", "error", err)
	}

	js, err := newJetstreamConnection()
	if err != nil {
		logger.Fatalw("failed to create NATS jetstream connection", "error", err)
	}

	server := &srv.Server{
		Context:         ctx,
		KubeClient:      client,
		Debug:           viper.GetBool("logging.debug"),
		Logger:          logger,
		Prefix:          viper.GetString("nats.subject-prefix"),
		ChartPath:       viper.GetString("chart-path"),
		JetstreamClient: js,
	}

	subjectPrefix := viper.GetString("nats.subject-prefix")
	if subjectPrefix == "" {
		logger.Fatalln("nats subject prefix is not set")
	}

	streamName := viper.GetString("nats.stream-name")
	if streamName == "" {
		logger.Fatalln("nats stream name is not set")
	}

	chart := viper.GetString("chart-path")
	if chart == "" {
		logger.Fatalln("no chart provided.")
	}

	subscription, err := js.QueueSubscribe(fmt.Sprintf("%s.>", subjectPrefix), "wallenda-workers", server.MessageHandler, nats.BindStream(streamName))
	if err != nil {
		logger.Errorf("unable to subscribe to queue: %s", err)
	}

	if err := server.ExposeEndpoint(subscription, viper.GetString("liveness-port")); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGABRT)

	// Wait for appropriate signal to trigger clean shutdown
	recvSig := <-sigCh
	signal.Stop(sigCh)
	logger.Infof("exiting with %s. Performing necessary cleanup", recvSig)

	return nil
}

func newJetstreamConnection() (nats.JetStreamContext, error) {
	opts := []nats.Option{}

	if viper.GetBool("development") {
		logger.Debug("enabling development settings")

		opts = append(opts, nats.Token(viper.GetString("nats.token")))
	} else {
		opt, err := nats.NkeyOptionFromSeed(viper.GetString("nats.nkey"))
		if err != nil {
			return nil, err
		}

		opts = append(opts, opt)
	}

	nc, err := nats.Connect(viper.GetString("nats.url"), opts...)
	if err != nil {
		return nil, err
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	return js, nil
}

func newKubeAuth(path string) (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Debugln("Unable to read in-cluster config")

		if path != "" {
			config, err = clientcmd.BuildConfigFromFlags("", path)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return config, nil
}
