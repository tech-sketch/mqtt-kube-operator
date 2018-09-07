package handlers

import (
	"fmt"

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

func (h *serviceHandler) apply(rawData runtime.Object) string {
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
			msg := fmt.Sprintf("update service err -- %s", name)
			h.logger.Errorf("%s: %s", msg, err.Error())
			return msg
		}
		msg := fmt.Sprintf("update service -- %s", name)
		h.logger.Infof(msg)
		return msg
	} else if errors.IsNotFound(getErr) {
		result, err := servicesClient.Create(service)
		if err != nil {
			msg := fmt.Sprintf("create service err -- %s", name)
			h.logger.Errorf("%s: %s", msg, err.Error())
			return msg
		}
		msg := fmt.Sprintf("create service -- %s", result.GetObjectMeta().GetName())
		h.logger.Infof(msg)
		return msg
	} else {
		msg := fmt.Sprintf("get service err -- %s", name)
		h.logger.Errorf("%s: %s", msg, getErr.Error())
		return msg
	}
}

func (h *serviceHandler) delete(rawData runtime.Object) string {
	service := rawData.(*apiv1.Service)
	servicesClient := h.kubeClient.CoreV1().Services(apiv1.NamespaceDefault)
	name := service.ObjectMeta.Name
	current, getErr := servicesClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		deletePolicy := metav1.DeletePropagationForeground
		if err := servicesClient.Delete(name, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			msg := fmt.Sprintf("delete service err -- %s", name)
			h.logger.Errorf("%s: %s", msg, err.Error())
			return msg
		}
		msg := fmt.Sprintf("delete service -- %s", name)
		h.logger.Infof(msg)
		return msg
	} else if errors.IsNotFound(getErr) {
		msg := fmt.Sprintf("service does not exist -- %s", name)
		h.logger.Infof(msg)
		return msg
	} else {
		msg := fmt.Sprintf("get service err -- %s", name)
		h.logger.Errorf("%s: %s", msg, getErr.Error())
		return msg
	}
}
