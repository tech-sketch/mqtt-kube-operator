/*
Package handlers : handle MQTT message and deploy object to kubernetes.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"testing"

	"github.com/ghodss/yaml"
	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/tech-sketch/mqtt-kube-operator/mock"
)

func setUpMocks(t *testing.T, deviceType string, deviceID string) (*MessageHandler, *mock.MockHandlerInf, *mock.MockHandlerInf, *mock.MockHandlerInf, *mock.MockHandlerInf, *mock.MockClient, *mock.MockMessage, *mock.MockToken, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	deployment := mock.NewMockHandlerInf(ctrl)
	service := mock.NewMockHandlerInf(ctrl)
	configmap := mock.NewMockHandlerInf(ctrl)
	secret := mock.NewMockHandlerInf(ctrl)

	handler := &MessageHandler{
		logger:           logger.Sugar(),
		deviceType:       deviceType,
		deviceID:         deviceID,
		deployment:       deployment,
		service:          service,
		configmap:        configmap,
		secret:           secret,
		sleepMillisecond: 0,
	}

	client := mock.NewMockClient(ctrl)
	message := mock.NewMockMessage(ctrl)
	token := mock.NewMockToken(ctrl)

	return handler, deployment, service, configmap, secret, client, message, token, func() {
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
				messageHandler := &MessageHandler{deviceType: deviceTypeCase.name, deviceID: deviceIDCase.name}

				assert.Equal(fmt.Sprintf("/%s/%s/cmd", deviceTypeCase.name, deviceIDCase.name), messageHandler.GetCmdTopic())
				assert.Equal(fmt.Sprintf("/%s/%s/cmdexe", deviceTypeCase.name, deviceIDCase.name), messageHandler.GetCmdExeTopic())
			})
		}
	}
}

func TestCommandInvalidPayload(t *testing.T) {
	messageHandler, deployment, service, configmap, secret, client, message, token, tearDown := setUpMocks(t, "dType", "dID")
	defer tearDown()

	payloadCases := []struct {
		payload string
		result  string
	}{
		{payload: "", result: "invalid payload"},
		{payload: "nil", result: "invalid payload"},
		{payload: "invalid", result: "invalid payload"},
		{payload: "@b|", result: "invalid payload"},
		{payload: "a@b|", result: "a@b|empty command body"},
		{payload: "a@b|%", result: "a@b|command body is invalid format"},
		{payload: "a@b|dummy", result: "a@b|unknown command"},
		{payload: "a@apply|{}", result: "a@apply|invalid format, skip this message"},
		{payload: "a@delete|dummy", result: "a@delete|invalid format, skip this message"},
	}

	for _, c := range payloadCases {
		t.Run(fmt.Sprintf("payload=%v", c.payload), func(t *testing.T) {
			var p []byte
			if c.payload != "nil" {
				p = []byte(c.payload)
			}
			message.EXPECT().Payload().Return(p)
			client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, c.result).Return(token)
			token.EXPECT().Wait().Return(false)
			deployment.EXPECT().Apply(gomock.Any()).Times(0)
			deployment.EXPECT().Delete(gomock.Any()).Times(0)
			service.EXPECT().Apply(gomock.Any()).Times(0)
			service.EXPECT().Delete(gomock.Any()).Times(0)
			configmap.EXPECT().Apply(gomock.Any()).Times(0)
			configmap.EXPECT().Delete(gomock.Any()).Times(0)
			secret.EXPECT().Apply(gomock.Any()).Times(0)
			secret.EXPECT().Delete(gomock.Any()).Times(0)

			messageHandler.Command()(client, message)
		})
	}
}

func TestDeployment(t *testing.T) {
	messageHandler, deployment, service, configmap, secret, client, message, token, tearDown := setUpMocks(t, "dType", "dID")
	defer tearDown()

	payload, rawData := getPayloadFromFixture(t, "../testdata/deployment.yaml")

	t.Run("apply", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@apply|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@apply|apply deployment success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(NewRawDataMatcher(rawData)).Return("apply deployment success").Times(1)
		deployment.EXPECT().Delete(gomock.Any()).Times(0)
		service.EXPECT().Apply(gomock.Any()).Times(0)
		service.EXPECT().Delete(gomock.Any()).Times(0)
		configmap.EXPECT().Apply(gomock.Any()).Times(0)
		configmap.EXPECT().Delete(gomock.Any()).Times(0)
		secret.EXPECT().Apply(gomock.Any()).Times(0)
		secret.EXPECT().Delete(gomock.Any()).Times(0)

		messageHandler.Command()(client, message)
	})

	t.Run("delete", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@delete|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@delete|delete deployment success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(gomock.Any()).Times(0)
		deployment.EXPECT().Delete(NewRawDataMatcher(rawData)).Return("delete deployment success").Times(1)
		service.EXPECT().Apply(gomock.Any()).Times(0)
		service.EXPECT().Delete(gomock.Any()).Times(0)
		configmap.EXPECT().Apply(gomock.Any()).Times(0)
		configmap.EXPECT().Delete(gomock.Any()).Times(0)
		secret.EXPECT().Apply(gomock.Any()).Times(0)
		secret.EXPECT().Delete(gomock.Any()).Times(0)

		messageHandler.Command()(client, message)
	})
}

func TestService(t *testing.T) {
	messageHandler, deployment, service, configmap, secret, client, message, token, tearDown := setUpMocks(t, "dType", "dID")
	defer tearDown()

	payload, rawData := getPayloadFromFixture(t, "../testdata/service.yaml")

	t.Run("apply", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@apply|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@apply|apply service success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(gomock.Any()).Times(0)
		deployment.EXPECT().Delete(gomock.Any()).Times(0)
		service.EXPECT().Apply(NewRawDataMatcher(rawData)).Return("apply service success").Times(1)
		service.EXPECT().Delete(gomock.Any()).Times(0)
		configmap.EXPECT().Apply(gomock.Any()).Times(0)
		configmap.EXPECT().Delete(gomock.Any()).Times(0)
		secret.EXPECT().Apply(gomock.Any()).Times(0)
		secret.EXPECT().Delete(gomock.Any()).Times(0)

		messageHandler.Command()(client, message)
	})

	t.Run("delete", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@delete|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@delete|delete service success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(gomock.Any()).Times(0)
		deployment.EXPECT().Delete(gomock.Any()).Times(0)
		service.EXPECT().Apply(gomock.Any()).Times(0)
		service.EXPECT().Delete(NewRawDataMatcher(rawData)).Return("delete service success").Times(1)
		configmap.EXPECT().Apply(gomock.Any()).Times(0)
		configmap.EXPECT().Delete(gomock.Any()).Times(0)
		secret.EXPECT().Apply(gomock.Any()).Times(0)
		secret.EXPECT().Delete(gomock.Any()).Times(0)

		messageHandler.Command()(client, message)
	})
}

func TestConfigmap(t *testing.T) {
	messageHandler, deployment, service, configmap, secret, client, message, token, tearDown := setUpMocks(t, "dType", "dID")
	defer tearDown()

	payload, rawData := getPayloadFromFixture(t, "../testdata/configmap.yaml")

	t.Run("apply", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@apply|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@apply|apply configmap success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(gomock.Any()).Times(0)
		deployment.EXPECT().Delete(gomock.Any()).Times(0)
		service.EXPECT().Apply(gomock.Any()).Times(0)
		service.EXPECT().Delete(gomock.Any()).Times(0)
		configmap.EXPECT().Apply(NewRawDataMatcher(rawData)).Return("apply configmap success").Times(1)
		configmap.EXPECT().Delete(gomock.Any()).Times(0)
		secret.EXPECT().Apply(gomock.Any()).Times(0)
		secret.EXPECT().Delete(gomock.Any()).Times(0)

		messageHandler.Command()(client, message)
	})

	t.Run("delete", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@delete|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@delete|delete configmap success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(gomock.Any()).Times(0)
		deployment.EXPECT().Delete(gomock.Any()).Times(0)
		service.EXPECT().Apply(gomock.Any()).Times(0)
		service.EXPECT().Delete(gomock.Any()).Times(0)
		configmap.EXPECT().Apply(gomock.Any()).Times(0)
		configmap.EXPECT().Delete(NewRawDataMatcher(rawData)).Return("delete configmap success").Times(1)
		secret.EXPECT().Apply(gomock.Any()).Times(0)
		secret.EXPECT().Delete(gomock.Any()).Times(0)

		messageHandler.Command()(client, message)
	})
}

func TestSecret(t *testing.T) {
	messageHandler, deployment, service, configmap, secret, client, message, token, tearDown := setUpMocks(t, "dType", "dID")
	defer tearDown()

	payload, rawData := getPayloadFromFixture(t, "../testdata/secret.yaml")

	t.Run("apply", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@apply|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@apply|apply secret success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(gomock.Any()).Times(0)
		deployment.EXPECT().Delete(gomock.Any()).Times(0)
		service.EXPECT().Apply(gomock.Any()).Times(0)
		service.EXPECT().Delete(gomock.Any()).Times(0)
		configmap.EXPECT().Apply(gomock.Any()).Times(0)
		configmap.EXPECT().Delete(gomock.Any()).Times(0)
		secret.EXPECT().Apply(NewRawDataMatcher(rawData)).Return("apply secret success").Times(1)
		secret.EXPECT().Delete(gomock.Any()).Times(0)

		messageHandler.Command()(client, message)
	})

	t.Run("delete", func(t *testing.T) {
		message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@delete|%s", url.QueryEscape(string(payload)))))
		client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, "a@delete|delete secret success").Return(token)
		token.EXPECT().Wait().Return(false)
		deployment.EXPECT().Apply(gomock.Any()).Times(0)
		deployment.EXPECT().Delete(gomock.Any()).Times(0)
		service.EXPECT().Apply(gomock.Any()).Times(0)
		service.EXPECT().Delete(gomock.Any()).Times(0)
		configmap.EXPECT().Apply(gomock.Any()).Times(0)
		configmap.EXPECT().Delete(gomock.Any()).Times(0)
		secret.EXPECT().Apply(gomock.Any()).Times(0)
		secret.EXPECT().Delete(NewRawDataMatcher(rawData)).Return("delete secret success").Times(1)

		messageHandler.Command()(client, message)
	})
}

func TestUnknownType(t *testing.T) {
	messageHandler, deployment, service, configmap, secret, client, message, token, tearDown := setUpMocks(t, "dType", "dID")
	defer tearDown()

	payload, _ := getPayloadFromFixture(t, "../testdata/namespace.yaml")

	methodCases := []struct {
		method string
	}{
		{method: "apply"},
		{method: "delete"},
	}

	for _, c := range methodCases {
		t.Run(c.method, func(t *testing.T) {
			message.EXPECT().Payload().Return([]byte(fmt.Sprintf("a@%s|%s", c.method, url.QueryEscape(string(payload)))))
			client.EXPECT().Publish("/dType/dID/cmdexe", byte(0), false, fmt.Sprintf("a@%s|unknown type, skip this message", c.method)).Return(token)
			token.EXPECT().Wait().Return(false)
			deployment.EXPECT().Apply(gomock.Any()).Times(0)
			deployment.EXPECT().Delete(gomock.Any()).Times(0)
			service.EXPECT().Apply(gomock.Any()).Times(0)
			service.EXPECT().Delete(gomock.Any()).Times(0)
			configmap.EXPECT().Apply(gomock.Any()).Times(0)
			configmap.EXPECT().Delete(gomock.Any()).Times(0)
			secret.EXPECT().Apply(gomock.Any()).Times(0)
			secret.EXPECT().Delete(gomock.Any()).Times(0)

			messageHandler.Command()(client, message)
		})
	}
}

func getPayloadFromFixture(t *testing.T, filepath string) ([]byte, runtime.Object) {
	yamlbytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}

	m := make(map[string]interface{})
	err = yaml.Unmarshal(yamlbytes, &m)
	if err != nil {
		t.Fatal(err)
	}
	jstr, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	rawData, _, err := decode(jstr, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	return jstr, rawData
}

type rawDataMatcher struct {
	rawData runtime.Object
}

func NewRawDataMatcher(rawData runtime.Object) gomock.Matcher {
	return &rawDataMatcher{
		rawData: rawData,
	}
}
func (m *rawDataMatcher) Matches(x interface{}) bool {
	switch d1 := m.rawData.(type) {
	case *appsv1.Deployment:
		d2, ok := x.(*appsv1.Deployment)
		if !ok {
			return false
		}
		if d1.ObjectMeta.Name != d2.ObjectMeta.Name {
			return false
		}
		return true
	case *apiv1.Service:
		d2, ok := x.(*apiv1.Service)
		if !ok {
			return false
		}
		if d1.ObjectMeta.Name != d2.ObjectMeta.Name {
			return false
		}
		return true
	case *apiv1.ConfigMap:
		d2, ok := x.(*apiv1.ConfigMap)
		if !ok {
			return false
		}
		if d1.ObjectMeta.Name != d2.ObjectMeta.Name {
			return false
		}
		return true
	case *apiv1.Secret:
		d2, ok := x.(*apiv1.Secret)
		if !ok {
			return false
		}
		if d1.ObjectMeta.Name != d2.ObjectMeta.Name {
			return false
		}
		return true
	default:
		return false
	}
}
func (m *rawDataMatcher) String() string {
	return ""
}
