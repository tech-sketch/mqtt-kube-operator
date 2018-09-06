FROM alpine:latest
MAINTAINER Nobuyuki Matsui <nobuyuki.matsui@gmail.com>

ARG CONTAINER_BINARY

ENV MQTT_TLS_CA_PATH "/etc/mqtt-kube-operator/certs/DST_Root_CA_X3.pem"
ENV MQTT_USERNAME ""
ENV MQTT_PASSWORD ""
ENV MQTT_HOST "localhost"
ENV MQTT_PORT "1883"
ENV MQTT_APPLY_TOPIC "/deployer/apply"
ENV MQTT_DELETE_TOPIC "/deployer/delete"

COPY ./$CONTAINER_BINARY /opt/mqtt-kube-operator
COPY ./certs/DST_Root_CA_X3.pem /etc/mqtt-kube-operator/certs/DST_Root_CA_X3.pem
WORKDIR /opt
ENTRYPOINT ["/opt/mqtt-kube-operator"]
