/*
Package reporters : report state of kubernetes using MQTT.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package reporters

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const deploymentAttrsFormat = "%s|deployment|%s|desired|%d|current|%d|updated|%d|ready|%d|unavailable|%d|available|%d"

/*
DeploymentStateReporter : a struct to report the state of Deployments.
*/
type DeploymentStateReporter struct {
	*baseReporter
	impl   ReporterImplInf
	logger *zap.SugaredLogger
}

/*
NewDeploymentStateReporter : a factory method to create DeploymentStateReporter.
*/
func NewDeploymentStateReporter(mqttClient mqtt.Client, kubeClient *kubernetes.Clientset, logger *zap.SugaredLogger, deviceType string, deviceID string, intervalSec int) *DeploymentStateReporter {
	return &DeploymentStateReporter{
		baseReporter: &baseReporter{deviceType, deviceID, time.Duration(intervalSec * 1000), make(chan bool, 1), make(chan bool, 1)},
		impl:         &deploymentStateReporterImpl{logger, mqttClient, kubeClient, time.Now},
		logger:       logger,
	}
}

/*
StartReporting : start a loop to report the state of PODs at the specified interval.
*/
func (r *DeploymentStateReporter) StartReporting() {
	go func() {
		r.logger.Debugf("start DeploymentStateReporter loop")
		r.baseReporter.loop(r.impl)
		r.logger.Debugf("stop DeploymentStateReporter loop")
	}()
}

type deploymentStateReporterImpl struct {
	logger         *zap.SugaredLogger
	mqttClient     mqtt.Client
	kubeClient     kubernetes.Interface
	getCurrentTime func() time.Time
}

func (impl *deploymentStateReporterImpl) Report(topic string) {
	impl.logger.Debugf("check deployments state")
	deploymentsClient := impl.kubeClient.AppsV1().Deployments(apiv1.NamespaceDefault)

	list, err := deploymentsClient.List(metav1.ListOptions{})
	if err != nil {
		impl.logger.Errorf("deploymentsClient list err -- %#v", err)
		return
	}
	for _, deployment := range list.Items {
		msg := fmt.Sprintf(deploymentAttrsFormat, impl.getCurrentTime().Format(time.RFC3339), deployment.ObjectMeta.Name,
			*deployment.Spec.Replicas, deployment.Status.Replicas, deployment.Status.UpdatedReplicas, deployment.Status.ReadyReplicas,
			deployment.Status.UnavailableReplicas, deployment.Status.AvailableReplicas)
		if token := impl.mqttClient.Publish(topic, 0, false, msg); token.Wait() && token.Error() != nil {
			impl.logger.Errorf("mqtt publish error, topic=%s, msg=%s, %s", topic, msg, token.Error())
		}
	}
}
