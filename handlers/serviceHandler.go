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

type serviceHandler struct {
	kubeClient *kubernetes.Clientset
	logger     *zap.SugaredLogger
}

func newServiceHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger) *serviceHandler {
	return &serviceHandler{
		kubeClient: clientset,
		logger:     logger,
	}
}

func (h *serviceHandler) apply(rawData runtime.Object) {
	service := rawData.(*apiv1.Service)
	servicesClient := h.kubeClient.CoreV1().Services(apiv1.NamespaceDefault)
	name := service.ObjectMeta.Name
	current, getErr := servicesClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			current.ObjectMeta.Labels = service.ObjectMeta.Labels
			current.ObjectMeta.Annotations = service.ObjectMeta.Annotations
			current.Spec = service.Spec
			_, err := servicesClient.Update(current)
			return err
		})
		if err != nil {
			h.logger.Errorf("update service err: %s\n", err.Error())
		}
		h.logger.Infof("update service %q\n", name)
	} else if errors.IsNotFound(getErr) {
		result, err := servicesClient.Create(service)
		if err != nil {
			h.logger.Errorf("create service err: %s\n", err.Error())
		}
		h.logger.Infof("create service %q\n", result.GetObjectMeta().GetName())
	} else {
		h.logger.Errorf("get service err: %s\n", getErr.Error())
	}
}

func (h *serviceHandler) delete(rawData runtime.Object) {
	service := rawData.(*apiv1.Service)
	servicesClient := h.kubeClient.CoreV1().Services(apiv1.NamespaceDefault)
	name := service.ObjectMeta.Name
	current, getErr := servicesClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		deletePolicy := metav1.DeletePropagationForeground
		if err := servicesClient.Delete(name, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			h.logger.Errorf("delete service err: %s\n", err.Error())
		}
		h.logger.Infof("delete service %q\n", name)
	} else if errors.IsNotFound(getErr) {
		h.logger.Infof("service does not exist: %s\n", name)
	} else {
		h.logger.Errorf("get service err: %s\n", getErr.Error())
	}
}
