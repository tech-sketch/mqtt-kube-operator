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
)

type executer struct {
	logger         *zap.SugaredLogger
	opts           *mqtt.ClientOptions
	cmdTopic       string
	messageHandler *handlers.MessageHandler
	client         mqtt.Client
}

func newExecuter(logger *zap.SugaredLogger) (*executer, error) {
	e := &executer{
		logger:   logger,
		opts:     mqtt.NewClientOptions(),
		cmdTopic: os.Getenv("MQTT_CMD_TOPIC"),
	}
	config, err := e.getKubeConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	e.messageHandler = handlers.NewMessageHandler(clientset, logger, e.cmdTopic)

	if err := e.setMQTTOptions(); err != nil {
		return nil, err
	}
	e.opts.OnConnect = e.onConnect
	e.client = mqtt.NewClient(e.opts)
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
		e.logger.Errorf("mqtt subscribe error, topic=%s, %s", e.cmdTopic, cmdToken.Error())
		panic(cmdToken.Error())
	}
}

func handle(e *executer) string {
	if token := e.client.Connect(); token.Wait() && token.Error() != nil {
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

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	exec, err := newExecuter(logger)
	if err != nil {
		logger.Errorf("executer error: %s", err.Error())
		panic(err)
	}

	handle(exec)
	<-c
	logger.Infof("finish main")
}
