package handlers

import (
	"go.uber.org/zap"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type configmapHandler struct {
	kubeClient *kubernetes.Clientset
	logger     *zap.SugaredLogger
}

func newConfigmapHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger) *configmapHandler {
	return &configmapHandler{
		kubeClient: clientset,
		logger:     logger,
	}
}

func (h *configmapHandler) apply(rawData runtime.Object) {
	configmap := rawData.(*apiv1.ConfigMap)
	configmapsClient := h.kubeClient.CoreV1().ConfigMaps(apiv1.NamespaceDefault)
	name := configmap.ObjectMeta.Name
	current, getErr := configmapsClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			current.ObjectMeta.Labels = configmap.ObjectMeta.Labels
			current.ObjectMeta.Annotations = configmap.ObjectMeta.Annotations
			current.Data = configmap.Data
			_, err := configmapsClient.Update(current)
			return err
		})
		if err != nil {
			h.logger.Errorf("update configmap err: %s\n", err.Error())
		}
		h.logger.Infof("update configmap %q\n", name)
	} else if errors.IsNotFound(getErr) {
		result, err := configmapsClient.Create(configmap)
		if err != nil {
			h.logger.Errorf("create configmap err: %s\n", err.Error())
		}
		h.logger.Infof("create configmap %q\n", result.GetObjectMeta().GetName())
	} else {
		h.logger.Errorf("get configmap err: %s\n", getErr.Error())
	}
}

func (h *configmapHandler) delete(rawData runtime.Object) {
	configmap := rawData.(*apiv1.ConfigMap)
	configmapsClient := h.kubeClient.CoreV1().ConfigMaps(apiv1.NamespaceDefault)
	name := configmap.ObjectMeta.Name
	current, getErr := configmapsClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		deletePolicy := metav1.DeletePropagationForeground
		if err := configmapsClient.Delete(name, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			h.logger.Errorf("delete configmap err: %s\n", err.Error())
		}
		h.logger.Infof("delete configmap %q\n", name)
	} else if errors.IsNotFound(getErr) {
		h.logger.Infof("configmap does not exist: %s\n", name)
	} else {
		h.logger.Errorf("get configmap err: %s\n", getErr.Error())
	}
}
