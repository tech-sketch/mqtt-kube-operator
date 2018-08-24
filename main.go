package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tech-sketch/mqtt-kube-operator/handlers"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
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
	log.Println("start main")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	config, err := getKubeConfig()
	if err != nil {
		log.Panicf("getConfig error: %s\n", err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicf("kubernetes.NewForConfig error: %s\n", err.Error())
	}
	messageHandler := handlers.NewMessageHandler(clientset).GetHandler()

	opts, err := getMQTTOptions()
	if err != nil {
		log.Panicf("getMQTTOptions error: %s\n", err.Error())
	}

	topic := os.Getenv("MQTT_TOPIC")
	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(topic, 0, messageHandler); token.Wait() && token.Error() != nil {
			log.Panicf("mqtt subscribe error: %s\n", token.Error())
		}
	}
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Panicf("mqtt connect error: %s\n", token.Error())
	} else {
		log.Println("Connected to server, start loop")
	}
	<-c
	log.Println("finish main")
}
