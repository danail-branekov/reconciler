package progress

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
	"sort"
)

const expectedReadyReplicas = 1
const expectedReadyDaemonSet = 1

func isDeploymentReady(ctx context.Context, client kubernetes.Interface, object *resource) (bool, error) {
	deployment, err := client.AppsV1().Deployments(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	replicaSet, err := getLatestReplicaSet(ctx, deployment, client.AppsV1())
	if err != nil || replicaSet == nil {
		return false, err
	}

	isReady := replicaSet.Status.ReadyReplicas >= expectedReadyReplicas
	return isReady, nil
}

func isStatefulSetReady(ctx context.Context, client kubernetes.Interface, object *resource) (bool, error) {
	statefulSet, err := client.AppsV1().StatefulSets(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	var partition, replicas = 0, 1
	if statefulSet.Spec.UpdateStrategy.RollingUpdate != nil && statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		partition = int(*statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)
	}

	if statefulSet.Spec.Replicas != nil {
		replicas = int(*statefulSet.Spec.Replicas)
	}

	expectedReplicas := replicas - partition
	if int(statefulSet.Status.UpdatedReplicas) != expectedReplicas {
		return false, nil
	}

	isReady := int(statefulSet.Status.ReadyReplicas) == replicas
	return isReady, nil
}

func isPodReady(ctx context.Context, client kubernetes.Interface, object *resource) (bool, error) {
	pod, err := client.CoreV1().Pods(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return false, nil
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			return false, nil
		}
	}
	//deletion timestamp determines whether pod is terminating or running (nil == running)
	return pod.ObjectMeta.DeletionTimestamp == nil, nil
}

func isDaemonSetReady(ctx context.Context, client kubernetes.Interface, object *resource) (bool, error) {
	daemonSet, err := client.AppsV1().DaemonSets(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if daemonSet.Status.UpdatedNumberScheduled != daemonSet.Status.DesiredNumberScheduled {
		return false, nil
	}

	isReady := int(daemonSet.Status.NumberReady) >= expectedReadyDaemonSet
	return isReady, nil
}

func isJobReady(ctx context.Context, client kubernetes.Interface, object *resource) (bool, error) {
	job, err := client.BatchV1().Jobs(object.namespace).Get(ctx, object.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			return false, nil
		}
	}
	return true, err
}

func getLatestReplicaSet(ctx context.Context, deployment *appsv1.Deployment, client appsclient.AppsV1Interface) (*appsv1.ReplicaSet, error) {
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}

	allReplicaSets, err := client.ReplicaSets(deployment.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	var ownedReplicaSets []*appsv1.ReplicaSet
	for i := range allReplicaSets.Items {
		if metav1.IsControlledBy(&allReplicaSets.Items[i], deployment) {
			ownedReplicaSets = append(ownedReplicaSets, &allReplicaSets.Items[i])
		}
	}

	if len(ownedReplicaSets) == 0 {
		return nil, nil
	}

	return findNewReplicaSet(deployment, ownedReplicaSets), nil
}

// findNewReplicaSet returns the new RS this given deployment targets (the one with the same pod template).
func findNewReplicaSet(deployment *appsv1.Deployment, rsList []*appsv1.ReplicaSet) *appsv1.ReplicaSet {
	sort.Sort(replicaSetsByCreationTimestamp(rsList))
	for i := range rsList {
		if equalIgnoreHash(&rsList[i].Spec.Template, &deployment.Spec.Template) {
			// In rare cases, such as after cluster upgrades, Deployment may end up with
			// having more than one new ReplicaSets that have the same template as its template,
			// see https://github.com/kubernetes/kubernetes/issues/40415
			// We deterministically choose the oldest new ReplicaSet.
			return rsList[i]
		}
	}
	// new ReplicaSet does not exist.
	return nil
}

func equalIgnoreHash(template1, template2 *corev1.PodTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	// Remove hash labels from template.Labels before comparing
	delete(t1Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(t2Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}

type replicaSetsByCreationTimestamp []*appsv1.ReplicaSet

func (o replicaSetsByCreationTimestamp) Len() int      { return len(o) }
func (o replicaSetsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o replicaSetsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}
