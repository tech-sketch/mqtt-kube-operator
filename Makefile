NAME=mqtt-kube-operator
VERSION=0.1.0

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOOS_CONTAINER=linux
GOARCH_CONTAINER=amd64
CONTAINER_BINARY=docker_$(NAME)
CONTAINER_IMAGE=techsketch/$(NAME)


all: clean deps test cross-compile docker-build
deps:
	@echo "---deps---"
	$(GOGET) k8s.io/client-go/...
	$(GOGET) github.com/eclipse/paho.mqtt.golang
	$(GOGET) go.uber.org/zap
test-deps:
	@echo "---test-deps---"
	$(GOGET) github.com/stretchr/testify
	$(GOGET) github.com/golang/mock/gomock
	$(GOGET) github.com/golang/mock/mockgen
	$(GOGET) github.com/ghodss/yaml
mock-gen:
	@echo "---mock-gen---"
	mockgen -destination mock/mock_clientset.go -package mock k8s.io/client-go/kubernetes Interface
	mockgen -destination mock/mock_corev1.go -package mock k8s.io/client-go/kubernetes/typed/core/v1 CoreV1Interface,ConfigMapInterface,SecretInterface,ServiceInterface
	mockgen -destination mock/mock_appsv1.go -package mock k8s.io/client-go/kubernetes/typed/apps/v1 AppsV1Interface,DeploymentInterface
	mockgen -destination mock/mock_mqtt.go -package mock github.com/eclipse/paho.mqtt.golang Client,Message,Token
	mockgen -destination mock/mock_handler.go -package mock -source handlers/interfaces.go
build:
	@echo "---build---"
	$(GOBUILD) -o $(NAME) -v
test: test-deps mock-gen
	@echo "---test---"
	go vet ./...
	golint ./...
	go test ./...
clean:
	@echo "---clean---"
	$(GOCLEAN)
	rm -f $(NAME)
	rm -f $(CONTAINER_BINARY)
	rm -rf mock/*.go
run:
	@echo "---run---"
	@echo "MQTT_USE_TLS=${MQTT_USE_TLS}"
	@echo "KUBE_CONF_PATH=${KUBE_CONF_PATH}"
	@echo "MQTT_TLS_CA_PATH=${MQTT_TLS_CA_PATH}"
	@echo "MQTT_USERNAME=${MQTT_USERNAME}"
	@echo "MQTT_PASSWORD=${MQTT_PASSWORD}"
	@echo "MQTT_HOST=${MQTT_HOST}"
	@echo "MQTT_PORT=${MQTT_PORT}"
	@echo "MQTT_CMD_TOPIC=${MQTT_CMD_TOPIC}"
	$(GOBUILD) -o $(NAME) -v
	./$(NAME)
cross-compile:
	@echo "---cross-compile---"
	GOOS=$(GOOS_CONTAINER) GOARCH=$(GOARCH_CONTAINER) $(GOBUILD) -o $(CONTAINER_BINARY) -v
docker-build:
	@echo "---docker-build---"
	docker build --build-arg CONTAINER_BINARY=$(CONTAINER_BINARY) -t $(CONTAINER_IMAGE):$(VERSION) .
push:
	@echo "---push---"
	docker push $(CONTAINER_IMAGE):$(VERSION)
