package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tech-sketch/mqtt-kube-operator/handlers"
)

func getKubeConfig() (*rest.Config, error) {
	kubeConfigPath := os.Getenv("KUBE_CONF_PATH")
	if kubeConfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	return rest.InClusterConfig()
}

func getMQTTOptions() (*mqtt.ClientOptions, error) {
	caPath := os.Getenv("MQTT_TLS_CA_PATH")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	host := os.Getenv("MQTT_HOST")
	port := os.Getenv("MQTT_PORT")

	ca, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("can not read '%q': %q", caPath, err.Error())
	}

	rootCA := x509.NewCertPool()
	if !rootCA.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("failed to parse root certificate: %q", caPath)
	}
	tlsConfig := &tls.Config{RootCAs: rootCA}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tls://%s:%s", host, port))
	opts.SetTLSConfig(tlsConfig)
	opts.SetClientID("kube-go")
	opts.SetCleanSession(true)
	opts.SetUsername(username)
	opts.SetPassword(password)

	return opts, nil
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

	config, err := getKubeConfig()
	if err != nil {
		logger.Errorf("getConfig error: %q", err.Error())
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Errorf("kubernetes.NewForConfig error: %q", err.Error())
		panic(err)
	}

	opts, err := getMQTTOptions()
	if err != nil {
		logger.Errorf("getMQTTOptions error: %q", err.Error())
		panic(err)
	}

	cmdTopic := os.Getenv("MQTT_CMD_TOPIC")
	messageHandler := handlers.NewMessageHandler(clientset, logger, cmdTopic)

	opts.OnConnect = func(c mqtt.Client) {
		if cmdToken := c.Subscribe(messageHandler.GetCmdTopic(), 0, messageHandler.Command()); cmdToken.Wait() && cmdToken.Error() != nil {
			logger.Errorf("mqtt subscribe error, topic=%q, %q", cmdTopic, cmdToken.Error())
			panic(cmdToken.Error())
		}
	}
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("mqtt connect error: %q", token.Error())
		panic(token.Error())
	} else {
		logger.Infof("Connected to server, start loop")
	}
	<-c
	logger.Infof("finish main")
}
