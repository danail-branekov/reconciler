package actions

import (
	"github.com/kyma-incubator/reconciler/pkg/reconciler/service"
	"github.com/pkg/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UninstallAction struct {
	name string
}

func NewUninstallAction() *UninstallAction {
	return &UninstallAction{name: "Uninstall Kpack"}
}

func (a *UninstallAction) Run(context *service.ActionContext) error {
	context.Logger.Infof("Uninstalling kpack...")
	kubeClient, err := context.KubeClient.Clientset()
	if err != nil {
		context.Logger.Errorf("Failed to retrieve native Kubernetes GO client")
	}

	policy := metav1.DeletePropagationForeground
	err = kubeClient.CoreV1().Namespaces().Delete(context.Context, kpackNamespace, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})

	if k8s_errors.IsNotFound(err) {
		context.Logger.Info("kpack is not installed, nothing to do")
		return nil
	}

	if err != nil {
		return errors.Wrap(err, "failed to uninstall kpack")
	}

	context.Logger.Info("kpack namespace deleted")
	return nil
}
