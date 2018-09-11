package handlers

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type HandlerInf interface {
	Apply(runtime.Object) string
	Delete(runtime.Object) string
}
