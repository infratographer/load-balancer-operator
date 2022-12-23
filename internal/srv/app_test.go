package srv

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"go.infratographer.com/loadbalanceroperator/internal/utils"
)

func TestNewHelmValues(t *testing.T) {
	type testCase struct {
		name        string
		valuesPath  string
		overrides   []valueSet
		expectError bool
	}

	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []testCase{
		{
			name:        "valid values path",
			expectError: false,
			valuesPath:  pwd + "/../../hack/ci/values.yaml",
			overrides:   nil,
		},
		{
			name:        "valid overrides",
			expectError: false,
			valuesPath:  pwd + "/../../hack/ci/values.yaml",
			overrides: []valueSet{
				{
					helmKey: "hello",
					value:   "world",
				},
			},
		},
		{
			name:        "missing values path",
			expectError: true,
			valuesPath:  "",
			overrides:   nil,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			srv := Server{
				Logger:     zap.NewNop().Sugar(),
				ValuesPath: tcase.valuesPath,
			}
			values, err := srv.newHelmValues(tcase.overrides)
			if tcase.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, values)
			}
		})
	}
}

func TestCreateNamespace(t *testing.T) {
	type testCase struct {
		name         string
		appNamespace string
		expectError  bool
		kubeclient   *rest.Config
	}

	env := envtest.Environment{}

	cfg, err := env.Start()
	if err != nil {
		panic(err)
	}

	testCases := []testCase{
		{
			name:         "valid yaml",
			expectError:  false,
			appNamespace: "flintlock",
			kubeclient:   cfg,
		},
		{
			name:         "invalid namespace",
			expectError:  true,
			appNamespace: "DarkwingDuck",
			kubeclient:   cfg,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			srv := Server{
				Context:    context.TODO(),
				Logger:     zap.NewNop().Sugar(),
				KubeClient: tcase.kubeclient,
			}

			err := srv.CreateNamespace(tcase.appNamespace)

			if tcase.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}

	err = env.Stop()
	if err != nil {
		panic(err)
	}
}

func TestNewDeployment(t *testing.T) {
	type testCase struct {
		name         string
		appNamespace string
		appName      string
		expectError  bool
		chart        *chart.Chart
		kubeClient   *rest.Config
		valPath      string
	}

	testDir, err := os.MkdirTemp("", "test-new-deployment")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(testDir)

	chartPath, err := utils.CreateTestChart(testDir)
	if err != nil {
		t.Fatal(err)
	}

	ch, err := loader.Load(chartPath)
	if err != nil {
		t.Fatal(err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	env := envtest.Environment{}

	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []testCase{
		{
			name:         "valid yaml",
			expectError:  false,
			appNamespace: uuid.New().String(),
			appName:      uuid.New().String(),
			chart:        ch,
			valPath:      pwd + "/../../hack/ci/values.yaml",
			kubeClient:   cfg,
		},
		{
			name:         "invalid namespace",
			expectError:  true,
			appNamespace: "DarkwingDuck",
			appName:      uuid.New().String(),
			chart:        ch,
			valPath:      pwd + "/../../hack/ci/values.yaml",
			kubeClient:   cfg,
		},
		{
			name:         "missing values path",
			expectError:  true,
			appNamespace: uuid.New().String(),
			appName:      uuid.New().String(),
			chart:        ch,
			valPath:      "",
			kubeClient:   cfg,
		},
		{
			name:         "invalid chart",
			expectError:  true,
			appNamespace: uuid.New().String(),
			appName:      uuid.New().String(),
			chart: &chart.Chart{
				Raw:       []*chart.File{},
				Metadata:  &chart.Metadata{},
				Lock:      &chart.Lock{},
				Templates: []*chart.File{},
				Values:    map[string]interface{}{},
				Schema:    []byte{},
				Files:     []*chart.File{},
			},
			valPath:    pwd + "/../../hack/ci/values.yaml",
			kubeClient: cfg,
		},
		{
			name:         "invalid helm client",
			expectError:  true,
			appNamespace: uuid.New().String(),
			appName:      uuid.New().String(),
			chart:        ch,
			valPath:      pwd + "/../../hack/ci/values.yaml",
			kubeClient:   nil,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			if err != nil {
				t.Fatal(err)
			}
			srv := Server{
				Context:    context.TODO(),
				Logger:     zap.NewNop().Sugar(),
				KubeClient: cfg,
				ValuesPath: pwd + "/../../hack/ci/values.yaml",
				Chart:      tcase.chart,
			}

			_ = srv.CreateNamespace(tcase.appNamespace)
			err = srv.newDeployment(tcase.appName, tcase.appNamespace, nil)

			if tcase.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}

	err = env.Stop()

	if err != nil {
		panic(err)
	}
}

func TestNewHelmClient(t *testing.T) {
	type testCase struct {
		name         string
		appNamespace string
		kubeClient   *rest.Config
		expectError  bool
	}

	env := envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []testCase{
		{
			name:         "valid client",
			appNamespace: "launchpad",
			kubeClient:   cfg,
			expectError:  false,
		},
		{
			name:         "invalid client",
			appNamespace: "glomgold",
			kubeClient:   nil,
			expectError:  true,
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			srv := Server{
				Context:    context.TODO(),
				Logger:     zap.NewNop().Sugar(),
				KubeClient: tcase.kubeClient,
			}

			_, err := srv.newHelmClient(tcase.appNamespace)

			if tcase.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
