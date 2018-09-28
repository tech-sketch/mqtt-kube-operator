/*
Package main : entry point of mqtt-kube-operator.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tech-sketch/mqtt-kube-operator/handlers"
	"github.com/tech-sketch/mqtt-kube-operator/reporters"
)

type executer struct {
	logger           *zap.SugaredLogger
	opts             *mqtt.ClientOptions
	deviceType       string
	deviceID         string
	messageHandler   *handlers.MessageHandler
	podStateReporter reporters.ReporterInf
	mqttClient       mqtt.Client
}

func newExecuter(logger *zap.SugaredLogger) (*executer, error) {
	e := &executer{
		logger:     logger,
		opts:       mqtt.NewClientOptions(),
		deviceType: os.Getenv("DEVICE_TYPE"),
		deviceID:   os.Getenv("DEVICE_ID"),
	}
	intervalSec, err := strconv.Atoi(os.Getenv("REPORT_INTERVAL_SEC"))
	if err != nil {
		intervalSec = 1
	}

	config, err := e.getKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	e.messageHandler = handlers.NewMessageHandler(clientset, logger, e.deviceType, e.deviceID)

	if err := e.setMQTTOptions(); err != nil {
		return nil, err
	}
	e.opts.OnConnect = e.onConnect
	e.mqttClient = mqtt.NewClient(e.opts)
	e.podStateReporter = reporters.NewPodStateReporter(e.mqttClient, clientset, logger, e.deviceType, e.deviceID, intervalSec)
	return e, nil
}

func (e *executer) getKubeConfig() (*rest.Config, error) {
	kubeConfigPath := os.Getenv("KUBE_CONF_PATH")
	if kubeConfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	return rest.InClusterConfig()
}

func (e *executer) setMQTTOptions() error {
	useTLS, err := strconv.ParseBool(os.Getenv("MQTT_USE_TLS"))
	if err != nil {
		useTLS = true
	}
	caPath := os.Getenv("MQTT_TLS_CA_PATH")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	host := os.Getenv("MQTT_HOST")
	port := os.Getenv("MQTT_PORT")

	if useTLS {
		ca, err := ioutil.ReadFile(caPath)
		if err != nil {
			return fmt.Errorf("can not read '%s': %s", caPath, err.Error())
		}

		rootCA := x509.NewCertPool()
		if !rootCA.AppendCertsFromPEM(ca) {
			return fmt.Errorf("failed to parse root certificate: %s", caPath)
		}
		tlsConfig := &tls.Config{RootCAs: rootCA}
		e.opts.AddBroker(fmt.Sprintf("tls://%s:%s", host, port))
		e.opts.SetTLSConfig(tlsConfig)
	} else {
		e.opts.AddBroker(fmt.Sprintf("tcp://%s:%s", host, port))
	}

	e.opts.SetClientID("kube-go")
	e.opts.SetCleanSession(true)
	e.opts.SetUsername(username)
	e.opts.SetPassword(password)

	return nil
}

func (e *executer) onConnect(c mqtt.Client) {
	if cmdToken := c.Subscribe(e.messageHandler.GetCmdTopic(), 0, e.messageHandler.Command()); cmdToken.Wait() && cmdToken.Error() != nil {
		e.logger.Errorf("mqtt subscribe error, deviceType=%s, deviceID=%s, %s", e.deviceType, e.deviceID, cmdToken.Error())
		panic(cmdToken.Error())
	}
	e.podStateReporter.StartReporting()
}

func handle(e *executer) string {
	if token := e.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		msg := fmt.Sprintf("mqtt connect error: %s", token.Error())
		e.logger.Errorf(msg)
		panic(token.Error())
	} else {
		msg := fmt.Sprintf("Connected to MQTT Broker(%s), start loop", e.opts.Servers[0].String())
		e.logger.Infof(msg)
		return msg
	}
}

func main() {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	logger := l.Sugar()
	defer logger.Sync()

	logger.Infof("start main")

	sigCh := make(chan os.Signal, 1)
	exitCh := make(chan bool, 1)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	exec, err := newExecuter(logger)
	if err != nil {
		logger.Errorf("executer error: %s", err.Error())
		panic(err)
	}
	handle(exec)

	go func() {
		s := <-sigCh
		logger.Infof("caught signal :%v", s)
		exec.podStateReporter.GetStopCh() <- true
		<-exec.podStateReporter.GetFinishCh()
		exitCh <- true
	}()

	<-exitCh
	logger.Infof("finish main")
}
