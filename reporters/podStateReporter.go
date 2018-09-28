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

const podAttrsFormat = "%s|podname|%s|podlabel|%s|podphase|%s"

/*
PodStateReporter : a struct to report the state of PODs.
*/
type PodStateReporter struct {
	*baseReporter
	impl   ReporterImplInf
	logger *zap.SugaredLogger
}

/*
NewPodStateReporter : a factory method to create PodStateReporter.
*/
func NewPodStateReporter(mqttClient mqtt.Client, kubeClient *kubernetes.Clientset, logger *zap.SugaredLogger, deviceType string, deviceID string, intervalSec int, targetLabelKey string) *PodStateReporter {
	return &PodStateReporter{
		baseReporter: &baseReporter{deviceType, deviceID, time.Duration(intervalSec * 1000), make(chan bool, 1), make(chan bool, 1)},
		impl:         &podStateReporterImpl{logger, mqttClient, kubeClient, targetLabelKey, time.Now},
		logger:       logger,
	}
}

/*
StartReporting : start a loop to report the state of PODs at the specified interval.
*/
func (r *PodStateReporter) StartReporting() {
	go func() {
		r.logger.Debugf("start PodStateReporter loop")
		r.baseReporter.loop(r.impl)
		r.logger.Debugf("stop PodStateReporter loop")
	}()
}

type podStateReporterImpl struct {
	logger         *zap.SugaredLogger
	mqttClient     mqtt.Client
	kubeClient     kubernetes.Interface
	targetLabelKey string
	getCurrentTime func() time.Time
}

func (impl *podStateReporterImpl) Report(topic string) {
	impl.logger.Debugf("check pods state")
	podsClient := impl.kubeClient.CoreV1().Pods(apiv1.NamespaceDefault)
	list, err := podsClient.List(metav1.ListOptions{})
	if err != nil {
		impl.logger.Errorf("podsClient list err -- %#v", err)
		return
	}
	for _, pod := range list.Items {
		if val, ok := pod.ObjectMeta.Labels[impl.targetLabelKey]; ok {
			msg := fmt.Sprintf(podAttrsFormat, impl.getCurrentTime().Format(time.RFC3339), pod.ObjectMeta.Name, val, pod.Status.Phase)
			if token := impl.mqttClient.Publish(topic, 0, false, msg); token.Wait() && token.Error() != nil {
				impl.logger.Errorf("mqtt publish error, topic=%s, msg=%s, %s", topic, msg, token.Error())
			}
		}
	}
}
