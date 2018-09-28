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
	logger      *zap.SugaredLogger
	mqttClient  mqtt.Client
	kubeClient  *kubernetes.Clientset
	intervalSec time.Duration
	stopCh      chan bool
	finishCh    chan bool
}

/*
NewPodStateReporter : a factory method to create PodStateReporter.
*/
func NewPodStateReporter(mqttClient mqtt.Client, kubeClient *kubernetes.Clientset, logger *zap.SugaredLogger, deviceType string, deviceID string, intervalSec int) *PodStateReporter {
	return &PodStateReporter{
		baseReporter: &baseReporter{deviceType, deviceID},
		logger:       logger,
		mqttClient:   mqttClient,
		kubeClient:   kubeClient,
		intervalSec:  time.Duration(intervalSec),
		stopCh:       make(chan bool, 1),
		finishCh:     make(chan bool, 1),
	}
}

/*
GetStopCh : get the channel to receive a loop stop message
*/
func (r *PodStateReporter) GetStopCh() chan bool {
	return r.stopCh
}

/*
GetFinishCh : get the channel to send a loop stopped message
*/
func (r *PodStateReporter) GetFinishCh() chan bool {
	return r.finishCh
}

/*
StartReporting : start a loop to report the state of PODs at the specified interval.
*/
func (r *PodStateReporter) StartReporting() {
	go func() {
		r.logger.Debugf("start reporter loop")
		ticker := time.NewTicker(r.intervalSec * time.Second)

	LOOP:
		for {
			select {
			case <-ticker.C:
				r.logger.Debugf("check pod status")
			case <-r.stopCh:
				ticker.Stop()
				break LOOP
			}
		}
		r.logger.Debugf("stop reporter loop")
		close(r.stopCh)
		close(r.finishCh)
	}()
}
