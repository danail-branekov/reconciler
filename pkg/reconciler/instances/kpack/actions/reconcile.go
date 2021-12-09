package actions

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/kpack/actions/interceptors"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	kpackChart     = "kpack"
	kpackNamespace = "kyma-kpack"
)

type ReconcileAction struct{}

func NewReconcileAction() *ReconcileAction {
	return &ReconcileAction{}
}

func (a *ReconcileAction) Run(context *service.ActionContext) error {
	context.Logger.Info("Reconciling kpack")

	desiredKpackVer, err := getKpackVersionFromChart(context)
	if err != nil {
		return errors.Wrap(err, "failed to get kpack version from chart")
	}

	client, err := context.KubeClient.Clientset()
	if err != nil {
		return errors.Wrap(err, "failed to get kuernetes client")
	}

	actualKpackVer, err := getInstalledKpackVersion(context.Context, client)
	if err != nil {
		return errors.Wrap(err, "failed to get installed kpack version")
	}

	if actualKpackVer == "" {
		context.Logger.Infof("No previous kpack found, installing version %s", desiredKpackVer)
		return installKpack(context)
	}

	canUpdate, err := isCompatible(actualKpackVer, desiredKpackVer)
	if err != nil {
		return errors.Wrapf(err, "failed to check compatibility between versions %s and %s", actualKpackVer, desiredKpackVer)
	}

	if actualKpackVer == desiredKpackVer {
		context.Logger.Infof("Kpack version %q has already been installed, nothing to do", desiredKpackVer)
		return nil
	}

	if canUpdate {
		context.Logger.Infof("Updating kpack from version %q to version %q", actualKpackVer, desiredKpackVer)
		return updateKpack(context)
	}

	return fmt.Errorf("not performing update due to different kpack major versions: actual %s vs desired %s", actualKpackVer, desiredKpackVer)
}

func getKpackVersionFromChart(context *service.ActionContext) (string, error) {
	workspace, err := context.WorkspaceFactory.Get(context.Task.Version)
	if err != nil {
		return "", err
	}

	chartBytes, err := ioutil.ReadFile(filepath.Join(workspace.ResourceDir, kpackChart, "Chart.yaml"))
	if err != nil {
		return "", err
	}

	var chartAttributes map[string]interface{}
	if err := yaml.Unmarshal(chartBytes, &chartAttributes); err != nil {
		return "", err
	}

	kpackVersion, ok := chartAttributes["appVersion"]
	if !ok {
		return "", errors.New("Kpack version is not defined in the Chart.yaml")
	}

	kpackVersionString, ok := kpackVersion.(string)
	if !ok {
		return "", fmt.Errorf("%v is not a string", kpackVersion)
	}

	return kpackVersionString, nil
}

func isCompatible(actual, desired string) (bool, error) {
	actualMajor, err := getMajor(actual)
	if err != nil {
		return false, err
	}

	desiredMajor, err := getMajor(actual)
	if err != nil {
		return false, err
	}

	return actualMajor == desiredMajor, nil
}

func getMajor(version string) (int, error) {
	var major, minor int
	var patch string
	_, err := fmt.Sscanf(version, "%d.%d.%s", &major, &minor, &patch)
	return major, errors.Wrapf(err, "failed to get major version from %s", version)
}

func installKpack(context *service.ActionContext) error {
	component := chart.NewComponentBuilder(context.Task.Version, kpackChart).
		WithNamespace(kpackNamespace).
		WithConfiguration(context.Task.Configuration).Build()

	manifest, err := context.ChartProvider.RenderManifest(component)
	if err != nil {
		return errors.Wrap(err, "failed to render manifest")
	}

	_, err = context.KubeClient.Deploy(context.Context, manifest.Manifest, kpackNamespace,
		&interceptors.ServicesInterceptor{
			KubeClient: context.KubeClient,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to deploy")
	}

	return nil
}

func updateKpack(context *service.ActionContext) error {
	return installKpack(context)
}

func getInstalledKpackVersion(ctx context.Context, k8sClient kubernetes.Interface) (string, error) {
	kpackPods, err := k8sClient.CoreV1().Pods(kpackNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=kpack-controller",
	})
	if err != nil {
		return "", err
	}

	if len(kpackPods.Items) == 0 {
		return "", nil
	}

	kpackVersion, ok := kpackPods.Items[0].Labels["version"]
	if !ok {
		return "", errors.New("no version label found on kpack-controller pods")
	}

	return kpackVersion, nil
}
