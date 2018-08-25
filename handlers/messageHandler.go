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

type handlerType int

const (
	_ handlerType = iota
	deploymentType
	serviceType
	configmapType
	secretType
)

func (h handlerType) String() string {
	switch h {
	case deploymentType:
		return "Deployment"
	case serviceType:
		return "Service"
	case configmapType:
		return "ConfigMap"
	case secretType:
		return "Secret"
	default:
		return "Unknown"
	}
}

type MessageHandler struct {
	logger     *zap.SugaredLogger
	deployment *deploymentHandler
	service    *serviceHandler
	configmap  *configmapHandler
	secret     *secretHandler
}

func NewMessageHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger) *MessageHandler {
	return &MessageHandler{
		logger:     logger,
		deployment: newDeploymentHandler(clientset, logger),
		service:    newServiceHandler(clientset, logger),
		configmap:  newConfigmapHandler(clientset, logger),
		secret:     newSecretHandler(clientset, logger),
	}
}

func (h *MessageHandler) Apply() mqtt.MessageHandler {
	operations := map[handlerType]func(runtime.Object){
		deploymentType: h.deployment.apply,
		serviceType:    h.service.apply,
		configmapType:  h.configmap.apply,
		secretType:     h.secret.apply,
	}
	return h.operate("received apply msg", operations)
}

func (h *MessageHandler) Delete() mqtt.MessageHandler {
	operations := map[handlerType]func(runtime.Object){
		deploymentType: h.deployment.delete,
		serviceType:    h.service.delete,
		configmapType:  h.configmap.delete,
		secretType:     h.secret.delete,
	}
	return h.operate("received delete msg", operations)
}

func (h *MessageHandler) operate(info string, operations map[handlerType]func(rawData runtime.Object)) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		h.logger.Infof("%s: %s\n", info, msg.Payload())
		decode := scheme.Codecs.UniversalDeserializer().Decode
		rawData, _, err := decode([]byte(msg.Payload()), nil, nil)
		if err != nil {
			h.logger.Infof("ignore format, skip this message: %s\n", err.Error())
		}
		switch rawData.(type) {
		case *appsv1.Deployment:
			operations[deploymentType](rawData)
		case *apiv1.Service:
			operations[serviceType](rawData)
		case *apiv1.ConfigMap:
			operations[configmapType](rawData)
		case *apiv1.Secret:
			operations[secretType](rawData)
		default:
			h.logger.Infof("unknown format, skip this message")
		}
	}
}
