/*
Package main : entry point of mqtt-kube-operator.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package main

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/tech-sketch/mqtt-kube-operator/handlers"
	"github.com/tech-sketch/mqtt-kube-operator/mock"
)

func setUpMocks(t *testing.T) (*executer, *mock.MockClient, *mock.MockToken, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	mqttClient := mock.NewMockClient(ctrl)
	token := mock.NewMockToken(ctrl)
	podStateReporter := mock.NewMockReporterInf(ctrl)

	exec := &executer{
		logger:           logger.Sugar(),
		mqttClient:       mqttClient,
		podStateReporter: podStateReporter,
	}
	return exec, mqttClient, token, func() {
		logger.Sync()
		ctrl.Finish()
	}
}

func TestSetMQTTOptions(t *testing.T) {
	assert := assert.New(t)
	exec, mqttClient, token, tearDown := setUpMocks(t)
	defer tearDown()

	useTLSCases := []struct {
		mqttUseTLS string
		caPath     string
	}{
		{mqttUseTLS: "false", caPath: ""},
		{mqttUseTLS: "False", caPath: ""},
		{mqttUseTLS: "true", caPath: "./certs/DST_Root_CA_X3.pem"},
		{mqttUseTLS: "nil", caPath: "./certs/DST_Root_CA_X3.pem"},
		{mqttUseTLS: "invalid", caPath: "./certs/DST_Root_CA_X3.pem"},
	}
	configCases := []struct {
		host     string
		port     string
		username string
		password string
	}{
		{host: "mqtt.example.com", port: "65534", username: "user", password: "passwd"},
		{host: "", port: "", username: "", password: ""},
		{host: "nil", port: "nil", username: "nil", password: "nil"},
	}

	for _, useTLSCase := range useTLSCases {
		for _, configCase := range configCases {
			t.Run(fmt.Sprintf("MQTT_USE_TLS=%v, MQTT_HOST=%v, MQTT_PORT=%v, MQTT_USERNAME=%v, MQTT_PASSWORD=%v",
				useTLSCase.mqttUseTLS, configCase.host, configCase.port, configCase.username, configCase.password), func(t *testing.T) {
				if useTLSCase.mqttUseTLS != "nil" {
					os.Setenv("MQTT_USE_TLS", useTLSCase.mqttUseTLS)
					defer os.Unsetenv("MQTT_USE_TLS")
				}
				if useTLSCase.caPath != "nil" {
					os.Setenv("MQTT_TLS_CA_PATH", useTLSCase.caPath)
					defer os.Unsetenv("MQTT_TLS_CA_PATH")
				}
				if configCase.host != "nil" {
					os.Setenv("MQTT_HOST", configCase.host)
					defer os.Unsetenv("MQTT_HOST")
				}
				if configCase.port != "nil" {
					os.Setenv("MQTT_PORT", configCase.port)
					defer os.Unsetenv("MQTT_PORT")
				}
				if configCase.username != "nil" {
					os.Setenv("MQTT_USERNAME", configCase.username)
					defer os.Unsetenv("MQTT_USERNAME")
				}
				if configCase.password != "nil" {
					os.Setenv("MQTT_PASSWORD", configCase.password)
					defer os.Unsetenv("MQTT_PASSWORD")
				}
				exec.opts = mqtt.NewClientOptions()
				err := exec.setMQTTOptions()

				assert.Nil(err)
				assert.NotNil(exec.opts)

				username := ""
				if configCase.username != "nil" {
					username = configCase.username
				}
				password := ""
				if configCase.password != "nil" {
					password = configCase.password
				}
				host := ""
				if configCase.host != "nil" {
					host = configCase.host
				}
				port := ""
				if configCase.port != "nil" {
					port = configCase.port
				}

				assert.Equal(username, exec.opts.Username)
				assert.Equal(password, exec.opts.Password)

				if useTLSCase.mqttUseTLS == "false" || useTLSCase.mqttUseTLS == "False" {
					assert.Equal(1, len(exec.opts.Servers))
					url, _ := url.Parse(fmt.Sprintf("tcp://%s:%s", host, port))
					assert.Equal(url, exec.opts.Servers[0])
					assert.Nil(exec.opts.TLSConfig.RootCAs)
				} else {
					assert.Equal(1, len(exec.opts.Servers))
					url, _ := url.Parse(fmt.Sprintf("tls://%s:%s", host, port))
					assert.Equal(url, exec.opts.Servers[0])
					assert.NotNil(exec.opts.TLSConfig.RootCAs)
				}

				mqttClient.EXPECT().Connect().Times(0)
				mqttClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				token.EXPECT().Wait().Times(0)
			})
		}
	}
}

func TestGetMQTTOptionsError(t *testing.T) {
	assert := assert.New(t)
	exec, mqttClient, token, tearDown := setUpMocks(t)
	defer tearDown()

	caCases := []struct {
		caPath string
	}{
		{caPath: "notexist"},
		{caPath: "nil"},
		{caPath: "./main.go"},
	}

	for _, caCase := range caCases {
		t.Run(fmt.Sprintf("MQTT_TLS_CA_PATH=%v", caCase.caPath), func(t *testing.T) {
			if caCase.caPath != "nil" {
				os.Setenv("MQTT_TLS_CA_PATH", caCase.caPath)
				defer os.Unsetenv("MQTT_TLS_CA_PATH")
			}
			os.Setenv("MQTT_USE_TLS", "true")
			defer os.Unsetenv("MQTT_USE_TLS")
			os.Setenv("MQTT_HOST", "mqtt.example.com")
			defer os.Unsetenv("MQTT_HOST")
			os.Setenv("MQTT_PORT", "8883")
			defer os.Unsetenv("MQTT_PORT")

			exec.opts = mqtt.NewClientOptions()
			err := exec.setMQTTOptions()
			assert.NotNil(err)

			switch caCase.caPath {
			case "notexist":
				assert.Equal("can not read 'notexist': open notexist: no such file or directory", err.Error())
			case "nil":
				assert.Equal("can not read '': open : no such file or directory", err.Error())
			case "./main.go":
				assert.Equal("failed to parse root certificate: ./main.go", err.Error())
			}

			mqttClient.EXPECT().Connect().Times(0)
			mqttClient.EXPECT().Subscribe(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			token.EXPECT().Wait().Times(0)
		})
	}
}

func TestHandle(t *testing.T) {
	assert := assert.New(t)
	exec, mqttClient, token, tearDown := setUpMocks(t)
	defer tearDown()

	exec.opts = mqtt.NewClientOptions()
	exec.opts.AddBroker("tcp://mqtt.example.com:1883")

	mqttClient.EXPECT().Connect().Return(token)
	token.EXPECT().Wait().Return(false)

	msg := handle(exec)
	assert.Equal("Connected to MQTT Broker(tcp://mqtt.example.com:1883), start loop", msg)
}

func TestOnConnect(t *testing.T) {
	exec, mqttClient, token, tearDown := setUpMocks(t)
	defer tearDown()

	exec.messageHandler = handlers.NewMessageHandler(nil, exec.logger, "testDeviceType", "testDeviceID")

	mqttClient.EXPECT().Subscribe("/testDeviceType/testDeviceID/cmd", byte(0), gomock.Any()).Return(token)
	token.EXPECT().Wait().Return(true)
	token.EXPECT().Error().Return(nil)
	exec.podStateReporter.(*mock.MockReporterInf).EXPECT().StartReporting()

	exec.onConnect(mqttClient)
}
