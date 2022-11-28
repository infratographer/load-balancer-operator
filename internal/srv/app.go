// Package srv provides server connectivity for wallenda
package srv

import (
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// CreateNamespace creates namespaces for the specified group that is
// provided in the event received
func (s *Server) CreateNamespace(groupID string) error {
	s.Logger.Debugf("ensuring namespace %s exists", groupID)
	kc, err := kubernetes.NewForConfig(s.KubeClient)

	if err != nil {
		s.Logger.Errorln("unable to authenticate against kubernetes cluster")
		return err
	}

	kind := "Namespace"
	apiv := "v1"
	apSpec := applyv1.NamespaceApplyConfiguration{
		TypeMetaApplyConfiguration: applymetav1.TypeMetaApplyConfiguration{
			Kind:       &kind,
			APIVersion: &apiv,
		},
		ObjectMetaApplyConfiguration: &applymetav1.ObjectMetaApplyConfiguration{
			Name: &groupID,
		},
		Spec:   &applyv1.NamespaceSpecApplyConfiguration{},
		Status: &applyv1.NamespaceStatusApplyConfiguration{},
	}
	_, err = kc.CoreV1().Namespaces().Apply(s.Context, &apSpec, metav1.ApplyOptions{FieldManager: "wallenda"})

	if err != nil {
		s.Logger.Errorf("unable to create namespace: %s", err)
		return err
	}

	return nil
}

// CreateApp deploys a loadBalancer based upon the configuration provided
// from the event that is processed.
func (s *Server) CreateApp(name string, chartPath string, namespace string) error {
	releaseName := fmt.Sprintf("%s-%s", name, namespace)

	chart, err := loader.Load(chartPath)
	if err != nil {
		s.Logger.Errorf("unable to load chart from %s", chartPath)
		return err
	}

	config := &action.Configuration{}
	cliopt := genericclioptions.NewConfigFlags(false)
	wrapper := func(*rest.Config) *rest.Config {
		return s.KubeClient
	}
	cliopt.WithWrapConfigFn(wrapper)

	err = config.Init(cliopt, namespace, "secret", func(format string, v ...interface{}) {
		// fmt.Println(v)

	})
	if err != nil {
		s.Logger.Errorln("unable to initialize helm client: %s", err)
		return err
	}

	hc := action.NewInstall(config)
	hc.ReleaseName = releaseName
	hc.Namespace = namespace
	_, err = hc.Run(chart, nil)

	if err != nil {
		s.Logger.Errorf("unable to deploy %s to %s", releaseName, namespace)
		return err
	}
	s.Logger.Infof("%s deployed to %s successfully", releaseName, namespace)

	return nil
}
