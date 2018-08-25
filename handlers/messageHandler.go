package handlers

import (
	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

type MessageHandler struct {
	logger     *zap.SugaredLogger
	deployment *deploymentHandler
	service    *serviceHandler
}

func NewMessageHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger) *MessageHandler {
	return &MessageHandler{
		logger:     logger,
		deployment: newDeploymentHandler(clientset, logger),
		service:    newServiceHandler(clientset, logger),
	}
}

func (h *MessageHandler) Apply() mqtt.MessageHandler {
	operations := map[string]func(runtime.Object){
		"deployment": h.deployment.apply,
		"service":    h.service.apply,
	}
	return h.operate("received apply msg", operations)
}

func (h *MessageHandler) Delete() mqtt.MessageHandler {
	operations := map[string]func(runtime.Object){
		"deployment": h.deployment.delete,
		"service":    h.service.delete,
	}
	return h.operate("received delete msg", operations)
}

func (h *MessageHandler) operate(info string, operations map[string]func(rawData runtime.Object)) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		h.logger.Infof("%s: %s\n", info, msg.Payload())
		decode := scheme.Codecs.UniversalDeserializer().Decode
		rawData, _, err := decode([]byte(msg.Payload()), nil, nil)
		if err != nil {
			h.logger.Infof("ignore format, skip this message: %s\n", err.Error())
		}
		switch rawData.(type) {
		case *appsv1.Deployment:
			operations["deployment"](rawData)
		case *apiv1.Service:
			operations["service"](rawData)
		default:
			h.logger.Infof("unknown format, skip this message")
		}
	}
}
