package main

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMQTTOptions(t *testing.T) {
	assert := assert.New(t)

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

				opts, err := getMQTTOptions()

				assert.Nil(err)
				assert.NotNil(opts)

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

				assert.Equal(username, opts.Username)
				assert.Equal(password, opts.Password)

				if useTLSCase.mqttUseTLS == "false" || useTLSCase.mqttUseTLS == "False" {
					assert.Equal(1, len(opts.Servers))
					url, _ := url.Parse(fmt.Sprintf("tcp://%s:%s", host, port))
					assert.Equal(url, opts.Servers[0])
					assert.Nil(opts.TLSConfig.RootCAs)
				} else {
					assert.Equal(1, len(opts.Servers))
					url, _ := url.Parse(fmt.Sprintf("tls://%s:%s", host, port))
					assert.Equal(url, opts.Servers[0])
					assert.NotNil(opts.TLSConfig.RootCAs)
				}
			})
		}
	}
}

func TestGetMQTTOptionsError(t *testing.T) {
	assert := assert.New(t)

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

			opts, err := getMQTTOptions()
			assert.Nil(opts)
			assert.NotNil(err)

			switch caCase.caPath {
			case "notexist":
				assert.Equal("can not read 'notexist': open notexist: no such file or directory", err.Error())
			case "nil":
				assert.Equal("can not read '': open : no such file or directory", err.Error())
			case "./main.go":
				assert.Equal("failed to parse root certificate: ./main.go", err.Error())
			}
		})
	}
}
