/*
Package reporters : report state of kubernetes using MQTT.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package reporters

import (
	"time"

	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"k8s.io/client-go/kubernetes"
)

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
func NewPodStateReporter(mqttClient mqtt.Client, kubeClient *kubernetes.Clientset, logger *zap.SugaredLogger, deviceType string, deviceID string, intervalSec int) *PodStateReporter {
	return &PodStateReporter{
		baseReporter: &baseReporter{deviceType, deviceID, time.Duration(intervalSec * 1000), make(chan bool, 1), make(chan bool, 1)},
		impl:         &podStateReporterImpl{logger, mqttClient, kubeClient},
		logger:       logger,
	}
}

/*
StartReporting : start a loop to report the state of PODs at the specified interval.
*/
func (r *PodStateReporter) StartReporting() {
	go func() {
		r.logger.Debugf("start reporter loop")
		r.baseReporter.loop(r.impl)
		r.logger.Debugf("stop reporter loop")
	}()
}

type podStateReporterImpl struct {
	logger     *zap.SugaredLogger
	mqttClient mqtt.Client
	kubeClient kubernetes.Interface
}

func (impl *podStateReporterImpl) Report() {
	impl.logger.Debugf("check pod state")
}
