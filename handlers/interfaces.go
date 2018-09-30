/*
Package handlers : handle MQTT message and deploy object to kubernetes.
	license: Apache license 2.0
	copyright: Nobuyuki Matsui <nobuyuki.matsui@gmail.com>
*/
package handlers

import (
	"k8s.io/apimachinery/pkg/runtime"
)

/*
HandlerInf : a interface to specify the method signatures that an object handler should be implemented.
*/
type HandlerInf interface {
	Apply(runtime.Object) string
	Delete(runtime.Object) string
}
