FROM alpine:latest
MAINTAINER Nobuyuki Matsui <nobuyuki.matsui@gmail.com>

ARG CONTAINER_BINARY

ENV MQTT_TLS_CA_PATH ""
ENV MQTT_USERNAME ""
ENV MQTT_PASSWORD ""
ENV MQTT_HOST "localhost"
ENV MQTT_PORT "1883"
ENV MQTT_TOPIC "/test"

COPY ./$CONTAINER_BINARY /opt/mqtt-kube-operator
WORKDIR /opt
ENTRYPOINT ["/opt/mqtt-kube-operator"]
