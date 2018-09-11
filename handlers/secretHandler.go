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

type secretHandler struct {
	kubeClient kubernetes.Interface
	logger     *zap.SugaredLogger
}

func newSecretHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger) *secretHandler {
	return &secretHandler{
		kubeClient: clientset,
		logger:     logger,
	}
}

func (h *secretHandler) Apply(rawData runtime.Object) string {
	secret := rawData.(*apiv1.Secret)
	secretsClient := h.kubeClient.CoreV1().Secrets(apiv1.NamespaceDefault)
	name := secret.ObjectMeta.Name
	current, getErr := secretsClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			current.ObjectMeta.Labels = secret.ObjectMeta.Labels
			current.ObjectMeta.Annotations = secret.ObjectMeta.Annotations
			current.Type = secret.Type
			current.Data = secret.Data
			_, err := secretsClient.Update(current)
			return err
		})
		if err != nil {
			msg := fmt.Sprintf("update secret err -- %s", name)
			h.logger.Errorf("%s: %s", msg, err.Error())
			return msg
		}
		msg := fmt.Sprintf("update secret -- %s", name)
		h.logger.Infof(msg)
		return msg
	} else if errors.IsNotFound(getErr) {
		result, err := secretsClient.Create(secret)
		if err != nil {
			msg := fmt.Sprintf("create secret err -- %s", name)
			h.logger.Errorf("%s: %s", msg, err.Error())
			return msg
		}
		msg := fmt.Sprintf("create secret -- %s", result.GetObjectMeta().GetName())
		h.logger.Infof(msg)
		return msg
	} else {
		msg := fmt.Sprintf("get secret err -- %s", name)
		h.logger.Errorf("%s: %s", msg, getErr.Error())
		return msg
	}
}

func (h *secretHandler) Delete(rawData runtime.Object) string {
	secret := rawData.(*apiv1.Secret)
	secretsClient := h.kubeClient.CoreV1().Secrets(apiv1.NamespaceDefault)
	name := secret.ObjectMeta.Name
	current, getErr := secretsClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		deletePolicy := metav1.DeletePropagationForeground
		if err := secretsClient.Delete(name, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			msg := fmt.Sprintf("delete secret err -- %s", name)
			h.logger.Errorf("%s: %s", msg, err.Error())
			return msg
		}
		msg := fmt.Sprintf("delete secret -- %s", name)
		h.logger.Infof(msg)
		return msg
	} else if errors.IsNotFound(getErr) {
		msg := fmt.Sprintf("secret does not exist -- %s", name)
		h.logger.Infof(msg)
		return msg
	} else {
		msg := fmt.Sprintf("get secret err -- %s", name)
		h.logger.Errorf("%s: %s", msg, getErr.Error())
		return msg
	}
}
