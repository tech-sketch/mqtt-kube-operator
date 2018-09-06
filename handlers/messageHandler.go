package handlers

import (
	"fmt"
	"net/url"
	"regexp"

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
	cmdTopic   string
	deployment *deploymentHandler
	service    *serviceHandler
	configmap  *configmapHandler
	secret     *secretHandler
}

func NewMessageHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger, cmdTopic string) *MessageHandler {
	return &MessageHandler{
		logger:     logger,
		cmdTopic:   cmdTopic,
		deployment: newDeploymentHandler(clientset, logger),
		service:    newServiceHandler(clientset, logger),
		configmap:  newConfigmapHandler(clientset, logger),
		secret:     newSecretHandler(clientset, logger),
	}
}

func (h *MessageHandler) GetCmdTopic() string {
	return h.cmdTopic + "/cmd"
}

func (h *MessageHandler) GetCmdExeTopic() string {
	return h.cmdTopic + "/cmdexe"
}

func (h *MessageHandler) Command() mqtt.MessageHandler {
	r := regexp.MustCompile(`^([\w\-]+)@([\w\-]+)\|(.*)$`)

	return func(client mqtt.Client, msg mqtt.Message) {
		h.logger.Infof("received message: %s\n", msg.Payload())
		g := r.FindSubmatch(msg.Payload())

		sendMessage := func(resultMsg string) {
			result := fmt.Sprintf("%s@%s|%s", g[1], g[2], resultMsg)
			if resultToken := client.Publish(h.GetCmdExeTopic(), 0, false, result); resultToken.Wait() && resultToken.Error() != nil {
				h.logger.Errorf("mqtt publish error, topic=%s, %s\n", "", resultToken.Error())
				panic(resultToken.Error())
			}
			h.logger.Infof("send message: %s\n", result)
		}

		if len(g[3]) == 0 {
			resultMsg := "empty command body"
			h.logger.Infof(resultMsg)
			sendMessage(resultMsg)
			return
		}
		data, err := url.QueryUnescape(string(g[3][:]))
		if err != nil {
			resultMsg := "command body is invalid format"
			h.logger.Infof(resultMsg)
			sendMessage(resultMsg)
			return
		}

		h.logger.Infof("data: %s", data)
		var resultMsg string
		switch string(g[2][:]) {
		case "apply":
			operations := map[handlerType]func(runtime.Object) string{
				deploymentType: h.deployment.apply,
				serviceType:    h.service.apply,
				configmapType:  h.configmap.apply,
				secretType:     h.secret.apply,
			}
			resultMsg = h.operate(operations, data)
		case "delete":
			operations := map[handlerType]func(runtime.Object) string{
				deploymentType: h.deployment.delete,
				serviceType:    h.service.delete,
				configmapType:  h.configmap.delete,
				secretType:     h.secret.delete,
			}
			resultMsg = h.operate(operations, data)
		default:
			resultMsg = "unknown command"
		}
		sendMessage(resultMsg)
	}
}

func (h *MessageHandler) operate(operations map[handlerType]func(rawData runtime.Object) string, data string) string {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	rawData, _, err := decode([]byte(data), nil, nil)
	if err != nil {
		msg := fmt.Sprintf("ignore format, skip this message: %s\n", err.Error())
		h.logger.Infof(msg)
		return msg
	}

	switch rawData.(type) {
	case *appsv1.Deployment:
		return operations[deploymentType](rawData)
	case *apiv1.Service:
		return operations[serviceType](rawData)
	case *apiv1.ConfigMap:
		return operations[configmapType](rawData)
	case *apiv1.Secret:
		return operations[secretType](rawData)
	default:
		msg := "unknown format, skip this message"
		h.logger.Infof(msg)
		return msg
	}
}
