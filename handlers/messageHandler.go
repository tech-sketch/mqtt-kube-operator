package handlers

import (
	"log"

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
}

func NewMessageHandler(clientset *kubernetes.Clientset) *MessageHandler {
	return &MessageHandler{
		kubeClient: clientset,
	}
}

func (messageHandler *MessageHandler) GetHandler() mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("received msg: %s\n", msg.Payload())
		decode := scheme.Codecs.UniversalDeserializer().Decode
		rawData, _, err := decode([]byte(msg.Payload()), nil, nil)
		if err != nil {
			log.Printf("ignore format, skip this message: %s\n", err.Error())
		}
		switch obj := rawData.(type) {
		case *appsv1.Deployment:
			messageHandler.applyDeployment(obj)
		default:
			log.Println("unknown format, skip this message")
		}
	}
}

func (messageHandler *MessageHandler) applyDeployment(deployment *appsv1.Deployment) {
	deploymentsClient := messageHandler.kubeClient.AppsV1().Deployments(apiv1.NamespaceDefault)
	name := deployment.ObjectMeta.Name
	_, getErr := deploymentsClient.Get(name, metav1.GetOptions{})

	if getErr == nil {
		result, err := deploymentsClient.Update(deployment)
		if err != nil {
			log.Panicf("update deployment err: %s\n", err.Error())
		}
		log.Printf("update deployment %q\n", result.GetObjectMeta().GetName())
	} else if errors.IsNotFound(getErr) {
		result, err := deploymentsClient.Create(deployment)
		if err != nil {
			log.Panicf("create deployment err: %s\n", err.Error())
		}
		log.Printf("created deployment %q\n", result.GetObjectMeta().GetName())
	} else {
		log.Panicf("get deployment err: %s\n", getErr.Error())
	}
}
