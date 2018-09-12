# mqtt-kube-operator
Deploy a resource to remote Kubernetes using MQTT

## Description
When this container is deployed a Kubernetes cluster, the container subscribes two MQTT topics.  
When a json string is received from subscribed topic, this container create / update / delete a Resource to its own Kubernetes.

## Limitations
* This program can operate only 4 resources below:

|resource|apiVersion|
|:--|:--|
|Deployment|apps/v1|
|Service|v1|
|ConfigMap|v1|
|Secret|v1|

* This program can operate only `default` namespace.

## Environment Variables
This REST API accept Environment Variables like below:

|Environment Variable|Summary|
|:--|:--|
|`MQTT_USE_TLS`|set `false` when connecting local MQTT Broker without TLS|
|`MQTT_TLS_CA_PATH`|path to cafile used to connect MQTT Broker|
|`MQTT_USERNAME`|username used to connect MQTT Broker|
|`MQTT_PASSWORD`|password used to connect MQTT Broker|
|`MQTT_HOST`|hostname of MQTT Broker|
|`MQTT_PORT`|port of MQTT Broker|
|`MQTT_CMD_TOPIC`|topic name used [iotagent-ul](https://github.com/telefonicaid/iotagent-ul) of [FIWARE](https://www.fiware.org)|
|`KUBE_CONF_PATH`|if set, run this program locally using kubectl's configuration|

## Run this program locally

1. set environment variables

    ```bash
    $ export KUBE_CONF_PATH=$HOME/.kube/config
    $ export MQTT_TLS_CA_PATH=/path/to/ca.crt
    $ export MQTT_USERNAME=mqtt_user
    $ export MQTT_PASSWORD=the_password_of_mqtt_user
    $ export MQTT_HOST=mqtt.example.com
    $ export MQTT_PORT=8883
    $ export MQTT_CMD_TOPIC=/deployer/delopyer_01
    ```
1. get dependencies (at the first time only)

    ```bash
    $ make deps
    ```
1. run program

    ```bash
    $ make run
    ```

## Build container from source code

1. build program and build container image

    ```bash
    $ make VERSION=0.1.0
    ```
1. push container to DockerHub

    ```bash
    $ docker login
    $ make push VERSION=0.1.0
    ```

## License

[Apache License 2.0](/LICENSE)

## Copyright
Copyright (c) 2018 TIS Inc.
