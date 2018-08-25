FROM alpine:latest
MAINTAINER Nobuyuki Matsui <nobuyuki.matsui@gmail.com>

ARG CONTAINER_BINARY

ENV MQTT_TLS_CA_PATH ""
ENV MQTT_USERNAME ""
ENV MQTT_PASSWORD ""
ENV MQTT_HOST "localhost"
ENV MQTT_PORT "1883"
ENV MQTT_APPLY_TOPIC "/deployer/apply"
ENV MQTT_DELETE_TOPIC "/deployer/delete"

COPY ./$CONTAINER_BINARY /opt/mqtt-kube-operator
WORKDIR /opt
ENTRYPOINT ["/opt/mqtt-kube-operator"]
