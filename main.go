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

var logger *zap.SugaredLogger

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	logger = l.Sugar()
}

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
		return nil, fmt.Errorf("can not read '%s': %s", caPath, err.Error())
	}

	rootCA := x509.NewCertPool()
	if !rootCA.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("failed to parse root certificate: %s", caPath)
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
	defer logger.Sync()
	logger.Infof("start main")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	config, err := getKubeConfig()
	if err != nil {
		logger.Errorf("getConfig error: %s\n", err.Error())
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Errorf("kubernetes.NewForConfig error: %s\n", err.Error())
		panic(err)
	}

	opts, err := getMQTTOptions()
	if err != nil {
		logger.Errorf("getMQTTOptions error: %s\n", err.Error())
		panic(err)
	}

	messageHandler := handlers.NewMessageHandler(clientset)
	defer messageHandler.Close()

	applyTopic := os.Getenv("MQTT_APPLY_TOPIC")
	deleteTopic := os.Getenv("MQTT_DELETE_TOPIC")
	opts.OnConnect = func(c mqtt.Client) {
		if applyToken := c.Subscribe(applyTopic, 0, messageHandler.Apply()); applyToken.Wait() && applyToken.Error() != nil {
			logger.Errorf("mqtt subscribe error, topic=%s, %s\n", applyTopic, applyToken.Error())
			panic(err)
		}
		if deleteToken := c.Subscribe(deleteTopic, 0, messageHandler.Delete()); deleteToken.Wait() && deleteToken.Error() != nil {
			logger.Errorf("mqtt subscribe error, topic=%s, %s\n", deleteTopic, deleteToken.Error())
			panic(err)
		}
	}
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Errorf("mqtt connect error: %s\n", token.Error())
		panic(err)
	} else {
		logger.Infof("Connected to server, start loop")
	}
	<-c
	logger.Infof("finish main")
}
