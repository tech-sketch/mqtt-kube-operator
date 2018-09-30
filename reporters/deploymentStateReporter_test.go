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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/tech-sketch/mqtt-kube-operator/mock"
)

func setUpDeploymentStateReporterMocks(t *testing.T, deviceType string, deviceID string, intervalSec int) (*DeploymentStateReporter, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	deploymentStateReporter := &DeploymentStateReporter{
		baseReporter: &baseReporter{deviceType, deviceID, time.Duration(intervalSec), make(chan bool, 1), make(chan bool, 1)},
		impl:         mock.NewMockReporterImplInf(ctrl),
		logger:       logger.Sugar(),
	}
	return deploymentStateReporter, func() {
		logger.Sync()
		ctrl.Finish()
	}
}

func TestDeploymentGetTopic(t *testing.T) {
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
				deploymentStateReporter, tearDown := setUpDeploymentStateReporterMocks(t, deviceTypeCase.name, deviceIDCase.name, 1)
				defer tearDown()

				assert.Equal(fmt.Sprintf("/%s/%s/attrs", deviceTypeCase.name, deviceIDCase.name), deploymentStateReporter.GetAttrsTopic())
			})
		}
	}
}

func TestDeploymentGetChannel(t *testing.T) {
	assert := assert.New(t)
	deploymentStateReporter, tearDown := setUpDeploymentStateReporterMocks(t, "dType", "dID", 1)
	defer tearDown()

	assert.Equal(deploymentStateReporter.stopCh, deploymentStateReporter.GetStopCh())
	assert.Equal(deploymentStateReporter.finishCh, deploymentStateReporter.GetFinishCh())
}

func TestDeploymentStartReporting(t *testing.T) {
	assert := assert.New(t)
	deploymentStateReporter, tearDown := setUpDeploymentStateReporterMocks(t, "dType", "dID", 10)
	defer tearDown()

	deploymentStateReporter.impl.(*mock.MockReporterImplInf).EXPECT().Report("/dType/dID/attrs").MinTimes(1)
	deploymentStateReporter.StartReporting()

	c := make(chan bool, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		deploymentStateReporter.GetStopCh() <- true
		assert.False(<-deploymentStateReporter.GetFinishCh())
		c <- true
	}()
	assert.True(<-c)
}

func setUpDeploymentStateReporterImplMocks(t *testing.T) (*deploymentStateReporterImpl, *mock.MockClient, *mock.MockToken, *mock.MockDeploymentInterface, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	mqttClient := mock.NewMockClient(ctrl)
	token := mock.NewMockToken(ctrl)

	kubeClient := mock.NewMockInterface(ctrl)
	apps := mock.NewMockAppsV1Interface(ctrl)
	deploymentsClient := mock.NewMockDeploymentInterface(ctrl)
	kubeClient.EXPECT().AppsV1().Return(apps).Times(1)
	apps.EXPECT().Deployments(gomock.Any()).Return(deploymentsClient).Times(1)

	impl := &deploymentStateReporterImpl{
		logger:     logger.Sugar(),
		mqttClient: mqttClient,
		kubeClient: kubeClient,
		getCurrentTime: func() time.Time {
			return time.Date(2018, 1, 2, 3, 4, 5, 0, time.Local)
		},
	}

	return impl, mqttClient, token, deploymentsClient, func() {
		logger.Sync()
		ctrl.Finish()
	}
}

func TestDeploymentReport(t *testing.T) {
	var desired1 int32 = 1
	var desired2 int32 = 11
	var desired3 int32 = 111
	testCases := []struct {
		deploymentList appsv1.DeploymentList
	}{
		{
			deploymentList: appsv1.DeploymentList{
				Items: []appsv1.Deployment{},
			},
		},
		{
			deploymentList: appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testdeployment1",
							Labels: map[string]string{"testkey": "value1", "dummy": "dummy"},
						},
						Spec: appsv1.DeploymentSpec{Replicas: &desired1},
						Status: appsv1.DeploymentStatus{
							Replicas:            2,
							UpdatedReplicas:     3,
							ReadyReplicas:       4,
							UnavailableReplicas: 5,
							AvailableReplicas:   6,
						},
					},
				},
			},
		},
		{
			deploymentList: appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testdeployment1",
							Labels: map[string]string{"testkey": "value1", "dummy": "dummy"},
						},
						Spec: appsv1.DeploymentSpec{Replicas: &desired1},
						Status: appsv1.DeploymentStatus{
							Replicas:            2,
							UpdatedReplicas:     3,
							ReadyReplicas:       4,
							UnavailableReplicas: 5,
							AvailableReplicas:   6,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testdeployment2",
							Labels: map[string]string{"testkey": "value2"},
						},
						Spec: appsv1.DeploymentSpec{Replicas: &desired2},
						Status: appsv1.DeploymentStatus{
							Replicas:            12,
							UpdatedReplicas:     13,
							ReadyReplicas:       14,
							UnavailableReplicas: 15,
							AvailableReplicas:   16,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testdeployment3",
							Labels: map[string]string{"dummy": "dummy"},
						},
						Spec: appsv1.DeploymentSpec{Replicas: &desired3},
						Status: appsv1.DeploymentStatus{
							Replicas:            112,
							UpdatedReplicas:     113,
							ReadyReplicas:       114,
							UnavailableReplicas: 115,
							AvailableReplicas:   116,
						},
					},
				},
			},
		},
	}
	testLabels := []struct {
		key string
	}{
		{key: "nil"},
		{key: ""},
		{key: "notexit"},
		{key: "testkey"},
	}

	dt := time.Date(2018, 1, 2, 3, 4, 5, 0, time.Local).Format(time.RFC3339)
	for _, testCase := range testCases {
		for _, testLabel := range testLabels {
			t.Run(fmt.Sprintf("deployment num=%d, label=%s", len(testCase.deploymentList.Items), testLabel.key), func(t *testing.T) {
				impl, mqttClient, token, deploymentsClient, tearDown := setUpDeploymentStateReporterImplMocks(t)
				defer tearDown()

				if testLabel.key != "nil" {
					impl.targetLabelKey = testLabel.key
				}

				deploymentsClient.EXPECT().List(gomock.Any()).Return(&testCase.deploymentList, nil).Times(1)
				if len(testCase.deploymentList.Items) == 0 {
					mqttClient.EXPECT().Publish("/test", byte(0), false, gomock.Any()).Times(0)
				} else if len(testCase.deploymentList.Items) == 1 {
					if testLabel.key == "testkey" {
						mqttClient.EXPECT().Publish("/test", byte(0), false, dt+"|deployment|testdeployment1|label|testkey:value1|desired|1|current|2|updated|3|ready|4|unavailable|5|available|6").Return(token).Times(1)
						token.EXPECT().Wait().Return(true).Times(1)
						token.EXPECT().Error().Return(nil).Times(1)
					} else {
						mqttClient.EXPECT().Publish("/test", byte(0), false, gomock.Any()).Times(0)
					}
				} else {
					if testLabel.key == "testkey" {
						gomock.InOrder(
							mqttClient.EXPECT().Publish("/test", byte(0), false, dt+"|deployment|testdeployment1|label|testkey:value1|desired|1|current|2|updated|3|ready|4|unavailable|5|available|6").Return(token),
							mqttClient.EXPECT().Publish("/test", byte(0), false, dt+"|deployment|testdeployment2|label|testkey:value2|desired|11|current|12|updated|13|ready|14|unavailable|15|available|16").Return(token),
						)
						token.EXPECT().Wait().Return(true).Times(2)
						token.EXPECT().Error().Return(nil).Times(2)
					} else {
						mqttClient.EXPECT().Publish("/test", byte(0), false, gomock.Any()).Times(0)
					}
				}
				impl.Report("/test")
			})
		}
	}
}

func TestDeploymentReportError(t *testing.T) {
	impl, mqttClient, _, deploymentsClient, tearDown := setUpDeploymentStateReporterImplMocks(t)
	defer tearDown()

	deploymentsClient.EXPECT().List(gomock.Any()).Return(nil, fmt.Errorf("test error")).Times(1)
	mqttClient.EXPECT().Publish("/test", byte(0), false, gomock.Any()).Times(0)

	impl.Report("/test")
}
