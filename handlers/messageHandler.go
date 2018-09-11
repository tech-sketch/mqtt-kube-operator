package handlers

import (
	"fmt"
	"net/url"
	"regexp"
	"time"

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
	logger           *zap.SugaredLogger
	cmdTopic         string
	deployment       HandlerInf
	service          HandlerInf
	configmap        HandlerInf
	secret           HandlerInf
	sleepMillisecond int
}

func NewMessageHandler(clientset *kubernetes.Clientset, logger *zap.SugaredLogger, cmdTopic string) *MessageHandler {
	return &MessageHandler{
		logger:           logger,
		cmdTopic:         cmdTopic,
		deployment:       newDeploymentHandler(clientset, logger),
		service:          newServiceHandler(clientset, logger),
		configmap:        newConfigmapHandler(clientset, logger),
		secret:           newSecretHandler(clientset, logger),
		sleepMillisecond: 500,
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
	publish := func(client mqtt.Client, payload string) {
		time.Sleep(time.Duration(h.sleepMillisecond) * time.Millisecond)
		if resultToken := client.Publish(h.GetCmdExeTopic(), 0, false, payload); resultToken.Wait() && resultToken.Error() != nil {
			h.logger.Errorf("mqtt publish error, topic=%s, %s", h.GetCmdExeTopic(), resultToken.Error())
			panic(resultToken.Error())
		}
		h.logger.Infof("send message: %s", payload)
	}

	return func(client mqtt.Client, msg mqtt.Message) {
		payload := msg.Payload()
		h.logger.Infof("received message: %s", payload)

		g := r.FindSubmatch(payload)

		if len(g) != 4 {
			publish(client, "invalid payload")
			return
		}

		sendMessage := func(resultMsg string) {
			result := fmt.Sprintf("%s@%s|%s", g[1], g[2], resultMsg)
			publish(client, result)
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
				deploymentType: h.deployment.Apply,
				serviceType:    h.service.Apply,
				configmapType:  h.configmap.Apply,
				secretType:     h.secret.Apply,
			}
			resultMsg = h.operate(operations, data)
		case "delete":
			operations := map[handlerType]func(runtime.Object) string{
				deploymentType: h.deployment.Delete,
				serviceType:    h.service.Delete,
				configmapType:  h.configmap.Delete,
				secretType:     h.secret.Delete,
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
		msg := "invalid format, skip this message"
		h.logger.Infof("%s: %s", msg, err.Error())
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
		msg := "unknown type, skip this message"
		h.logger.Infof(msg)
		return msg
	}
}
