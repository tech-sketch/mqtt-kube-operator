package handlers

import (
	"fmt"
	"testing"

	"go.uber.org/zap"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/tech-sketch/mqtt-kube-operator/mock"
)

func setUpServiceHandler(t *testing.T) (*serviceHandler, *mock.MockServiceInterface, runtime.Object, *apiv1.Service, string, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	clientset := mock.NewMockInterface(ctrl)
	corev1 := mock.NewMockCoreV1Interface(ctrl)
	service := mock.NewMockServiceInterface(ctrl)
	clientset.EXPECT().CoreV1().Return(corev1).AnyTimes()
	corev1.EXPECT().Services(gomock.Any()).Return(service).AnyTimes()

	handler := &serviceHandler{
		kubeClient: clientset,
		logger:     logger.Sugar(),
	}

	_, rawData := getPayloadFromFixture(t, "../testdata/service.yaml")
	obj, ok := rawData.(*apiv1.Service)
	if !ok {
		t.Fatal(ok)
	}

	name := "my-service"

	return handler, service, rawData, obj, name, func() {
		logger.Sync()
		ctrl.Finish()
	}
}

func TestServiceCreate(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpServiceHandler(t)
	defer tearDown()

	getErr := errors.NewNotFound(apiv1.Resource("service"), name)

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, getErr)
		client.EXPECT().Create(obj).Return(obj, nil)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("create service -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, getErr)
		client.EXPECT().Create(obj).Return(nil, fmt.Errorf("failure"))
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("create service err -- %s", name), result)
	})
}

func TestServiceUpdate(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpServiceHandler(t)
	defer tearDown()

	prev := rawData.(*apiv1.Service)
	prev.ObjectMeta.Labels["test"] = "test"

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(prev, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(obj).Return(nil, nil)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("update service -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(prev, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(obj).Return(nil, fmt.Errorf("failure"))
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("update service err -- %s", name), result)
	})
}

func TestServiceApplyGetErr(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, _, name, tearDown := setUpServiceHandler(t)
	defer tearDown()

	client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, fmt.Errorf("getErr"))
	client.EXPECT().Create(gomock.Any()).Times(0)
	client.EXPECT().Update(gomock.Any()).Times(0)
	client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	result := handler.Apply(rawData)
	assert.Equal(fmt.Sprintf("get service err -- %s", name), result)
}

func TestServiceDelete(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpServiceHandler(t)
	defer tearDown()

	deletePolicy := metav1.DeletePropagationForeground

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(obj, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy}).Return(nil)

		result := handler.Delete(rawData)
		assert.Equal(fmt.Sprintf("delete service -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(obj, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy}).Return(fmt.Errorf("failure"))

		result := handler.Delete(rawData)
		assert.Equal(fmt.Sprintf("delete service err -- %s", name), result)
	})
}

func TestServiceDeleteError(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, _, name, tearDown := setUpServiceHandler(t)
	defer tearDown()

	errCases := []struct {
		name string
		err  error
		msg  string
	}{
		{name: "notfound", err: errors.NewNotFound(apiv1.Resource("service"), name), msg: "service does not exist"},
		{name: "othererr", err: fmt.Errorf("failure"), msg: "get service err"},
	}

	for _, c := range errCases {
		t.Run(c.name, func(t *testing.T) {
			client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, c.err)
			client.EXPECT().Create(gomock.Any()).Times(0)
			client.EXPECT().Update(gomock.Any()).Times(0)
			client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

			result := handler.Delete(rawData)
			assert.Equal(fmt.Sprintf("%s -- %s", c.msg, name), result)
		})
	}
}
