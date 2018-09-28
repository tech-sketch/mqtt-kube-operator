/*
Package reporters : report state of kubernetes using MQTT.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package reporters

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/tech-sketch/mqtt-kube-operator/mock"
)

func setUpPodStateReporterMocks(t *testing.T, deviceType string, deviceID string, intervalSec int) (*PodStateReporter, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	podStateReporter := &PodStateReporter{
		baseReporter: &baseReporter{deviceType, deviceID, time.Duration(intervalSec), make(chan bool, 1), make(chan bool, 1)},
		impl:         mock.NewMockReporterImplInf(ctrl),
		logger:       logger.Sugar(),
	}
	return podStateReporter, func() {
		logger.Sync()
		ctrl.Finish()
	}
}

func TestGetTopic(t *testing.T) {
	assert := assert.New(t)

	deviceTypeCases := []struct {
		name string
	}{
		{name: "dType"},
		{name: "/"},
		{name: ""},
	}

	deviceIDCases := []struct {
		name string
	}{
		{name: "dID"},
		{name: "/"},
		{name: ""},
	}

	for _, deviceTypeCase := range deviceTypeCases {
		for _, deviceIDCase := range deviceIDCases {
			t.Run(fmt.Sprintf("deviceType=%v, deviceID=%v", deviceTypeCase.name, deviceIDCase.name), func(t *testing.T) {
				podStateReporter, tearDown := setUpPodStateReporterMocks(t, deviceTypeCase.name, deviceIDCase.name, 1)
				defer tearDown()

				assert.Equal(fmt.Sprintf("/%s/%s/attrs", deviceTypeCase.name, deviceIDCase.name), podStateReporter.GetAttrsTopic())
			})
		}
	}
}

func TestGetChannel(t *testing.T) {
	assert := assert.New(t)
	podStateReporter, tearDown := setUpPodStateReporterMocks(t, "dType", "dID", 1)
	defer tearDown()

	assert.Equal(podStateReporter.stopCh, podStateReporter.GetStopCh())
	assert.Equal(podStateReporter.finishCh, podStateReporter.GetFinishCh())
}

func TestStartReporting(t *testing.T) {
	assert := assert.New(t)
	podStateReporter, tearDown := setUpPodStateReporterMocks(t, "dType", "dID", 10)
	defer tearDown()

	podStateReporter.impl.(*mock.MockReporterImplInf).EXPECT().Report().MinTimes(1)
	podStateReporter.StartReporting()

	c := make(chan bool, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		podStateReporter.GetStopCh() <- true
		assert.False(<-podStateReporter.GetFinishCh())
		c <- true
	}()
	assert.True(<-c)
}
