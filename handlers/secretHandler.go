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

type secretHandler struct {
	kubeClient *kubernetes.Clientset
	logger     *zap.SugaredLogger
}

func newSecretHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger) *secretHandler {
	return &secretHandler{
		kubeClient: clientset,
		logger:     logger,
	}
}

func (h *secretHandler) apply(rawData runtime.Object) {
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
			h.logger.Errorf("update secret err: %s\n", err.Error())
		}
		h.logger.Infof("update secret %q\n", name)
	} else if errors.IsNotFound(getErr) {
		result, err := secretsClient.Create(secret)
		if err != nil {
			h.logger.Errorf("create secret err: %s\n", err.Error())
		}
		h.logger.Infof("create secret %q\n", result.GetObjectMeta().GetName())
	} else {
		h.logger.Errorf("get secret err: %s\n", getErr.Error())
	}
}

func (h *secretHandler) delete(rawData runtime.Object) {
	secret := rawData.(*apiv1.Secret)
	secretsClient := h.kubeClient.CoreV1().Secrets(apiv1.NamespaceDefault)
	name := secret.ObjectMeta.Name
	current, getErr := secretsClient.Get(name, metav1.GetOptions{})

	if current != nil && getErr == nil {
		deletePolicy := metav1.DeletePropagationForeground
		if err := secretsClient.Delete(name, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			h.logger.Errorf("delete secret err: %s\n", err.Error())
		}
		h.logger.Infof("delete secret %q\n", name)
	} else if errors.IsNotFound(getErr) {
		h.logger.Infof("secret does not exist: %s\n", name)
	} else {
		h.logger.Errorf("get secret err: %s\n", getErr.Error())
	}
}
