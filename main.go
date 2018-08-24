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

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

var clientset *kubernetes.Clientset
var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("received msg: %s\n", msg.Payload())
	decode := scheme.Codecs.UniversalDeserializer().Decode
	rawData, _, err := decode([]byte(msg.Payload()), nil, nil)
	if err != nil {
		log.Printf("ignore format, skip this message: %s\n", err.Error())
	}
	switch obj := rawData.(type) {
	case *appsv1.Deployment:
		deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
		name := obj.ObjectMeta.Name
		_, getErr := deploymentsClient.Get(name, metav1.GetOptions{})

		if getErr == nil {
			result, err := deploymentsClient.Update(obj)
			if err != nil {
				log.Panicf("update deployment err: %s\n", err.Error())
			}
			log.Printf("update deployment %q\n", result.GetObjectMeta().GetName())
		} else if errors.IsNotFound(getErr) {
			result, err := deploymentsClient.Create(obj)
			if err != nil {
				log.Panicf("create deployment err: %s\n", err.Error())
			}
			log.Printf("created deployment %q\n", result.GetObjectMeta().GetName())
		} else {
			log.Panicf("get deployment err: %s\n", getErr.Error())
		}
	default:
		log.Println("unknown format, skip this message")
	}
}

func getConfig() (*rest.Config, error) {
	kubeConfigPath := os.Getenv("KUBE_CONF_PATH")
	if kubeConfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	}
	return rest.InClusterConfig()
}

func main() {
	log.Println("start main")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	config, err := getConfig()
	if err != nil {
		log.Panicf("getConfig error: %s\n", err.Error())
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicf("kubernetes.NewForConfig error: %s\n", err.Error())
	}

	caPath := os.Getenv("MQTT_TLS_CA_PATH")
	username := os.Getenv("MQTT_USERNAME")
	password := os.Getenv("MQTT_PASSWORD")
	host := os.Getenv("MQTT_HOST")
	port := os.Getenv("MQTT_PORT")
	topic := os.Getenv("MQTT_TOPIC")

	ca, err := ioutil.ReadFile(caPath)
	if err != nil {
		log.Panicf("can not read '%s': %s\n", caPath, err.Error())
	}

	rootCA := x509.NewCertPool()
	if !rootCA.AppendCertsFromPEM(ca) {
		log.Panicf("failed to parse root certificate: %s\n", caPath)
	}
	tlsConfig := &tls.Config{RootCAs: rootCA}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tls://%s:%s", host, port))
	opts.SetTLSConfig(tlsConfig)
	opts.SetClientID("kube-go")
	opts.SetCleanSession(true)
	opts.SetUsername(username)
	opts.SetPassword(password)

	opts.OnConnect = func(c mqtt.Client) {
		if token := c.Subscribe(topic, 0, f); token.Wait() && token.Error() != nil {
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
