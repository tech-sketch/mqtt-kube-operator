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


all: deps test cross-compile docker-build
deps:
	@echo "---deps---"
	$(GOGET) k8s.io/client-go/...
	$(GOGET) github.com/eclipse/paho.mqtt.golang
	$(GOGET) go.uber.org/zap
build:
	@echo "---build---"
	$(GOBUILD) -o $(NAME) -v
test:
	@echo "---test---"
	$(GOGET) github.com/stretchr/testify
	go test ./...
clean:
	@echo "---clean---"
	$(GOCLEAN)
	rm -f $(NAME)
	rm -f $(CONTAINER_BINARY)
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
