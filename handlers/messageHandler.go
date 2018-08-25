package handlers

import (
	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

type MessageHandler struct {
	kubeClient *kubernetes.Clientset
	logger     *zap.SugaredLogger
}

func NewMessageHandler(clientset *kubernetes.Clientset) *MessageHandler {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	logger := l.Sugar()

	return &MessageHandler{
		kubeClient: clientset,
		logger:     logger,
	}
}

func (h *MessageHandler) Close() {
	h.logger.Sync()
}

func (h *MessageHandler) Apply() mqtt.MessageHandler {
	return h.operate("received apply msg", h.applyDeployment)
}

func (h *MessageHandler) Delete() mqtt.MessageHandler {
	return h.operate("received delete msg", h.deleteDeployment)
}

func (h *MessageHandler) operate(info string, operation func(deployment *appsv1.Deployment)) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		h.logger.Infof("%s: %s\n", info, msg.Payload())
		decode := scheme.Codecs.UniversalDeserializer().Decode
		rawData, _, err := decode([]byte(msg.Payload()), nil, nil)
		if err != nil {
			h.logger.Infof("ignore format, skip this message: %s\n", err.Error())
		}
		switch obj := rawData.(type) {
		case *appsv1.Deployment:
			operation(obj)
		default:
			h.logger.Infof("unknown format, skip this message")
		}
	}
}

func (h *MessageHandler) applyDeployment(deployment *appsv1.Deployment) {
	deploymentsClient := h.kubeClient.AppsV1().Deployments(apiv1.NamespaceDefault)
	name := deployment.ObjectMeta.Name
	_, getErr := deploymentsClient.Get(name, metav1.GetOptions{})

	if getErr == nil {
		result, err := deploymentsClient.Update(deployment)
		if err != nil {
			h.logger.Errorf("update deployment err: %s\n", err.Error())
		}
		h.logger.Infof("update deployment %q\n", result.GetObjectMeta().GetName())
	} else if errors.IsNotFound(getErr) {
		result, err := deploymentsClient.Create(deployment)
		if err != nil {
			h.logger.Errorf("create deployment err: %s\n", err.Error())
		}
		h.logger.Infof("create deployment %q\n", result.GetObjectMeta().GetName())
	} else {
		h.logger.Errorf("get deployment err: %s\n", getErr.Error())
	}
}

func (h *MessageHandler) deleteDeployment(deployment *appsv1.Deployment) {
	deploymentsClient := h.kubeClient.AppsV1().Deployments(apiv1.NamespaceDefault)
	name := deployment.ObjectMeta.Name
	_, getErr := deploymentsClient.Get(name, metav1.GetOptions{})

	if getErr == nil {
		deletePolicy := metav1.DeletePropagationForeground
		if err := deploymentsClient.Delete(name, &metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}); err != nil {
			h.logger.Errorf("delete deployment err: %s\n", err.Error())
		}
		h.logger.Infof("delete deployment %q\n", name)
	} else if errors.IsNotFound(getErr) {
		h.logger.Warnf("deployment does not exist: %s\n", name)
	} else {
		h.logger.Errorf("get deployment err: %s\n", getErr.Error())
	}
}
