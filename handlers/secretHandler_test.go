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

func setUpSecretHandler(t *testing.T) (*secretHandler, *mock.MockSecretInterface, runtime.Object, *apiv1.Secret, string, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	clientset := mock.NewMockInterface(ctrl)
	corev1 := mock.NewMockCoreV1Interface(ctrl)
	secret := mock.NewMockSecretInterface(ctrl)
	clientset.EXPECT().CoreV1().Return(corev1).AnyTimes()
	corev1.EXPECT().Secrets(gomock.Any()).Return(secret).AnyTimes()

	handler := &secretHandler{
		kubeClient: clientset,
		logger:     logger.Sugar(),
	}

	_, rawData := getPayloadFromFixture(t, "../testdata/secret.yaml")
	obj, ok := rawData.(*apiv1.Secret)
	if !ok {
		t.Fatal(ok)
	}

	name := "my-secret"

	return handler, secret, rawData, obj, name, func() {
		logger.Sync()
		ctrl.Finish()
	}
}

func TestSecretCreate(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpSecretHandler(t)
	defer tearDown()

	getErr := errors.NewNotFound(apiv1.Resource("secret"), name)

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, getErr)
		client.EXPECT().Create(obj).Return(obj, nil)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("create secret -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, getErr)
		client.EXPECT().Create(obj).Return(nil, fmt.Errorf("failure"))
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("create secret err -- %s", name), result)
	})
}

func TestSecretUpdate(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpSecretHandler(t)
	defer tearDown()

	prev := rawData.(*apiv1.Secret)
	prev.ObjectMeta.Labels["test"] = "test"

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(prev, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(obj).Return(nil, nil)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("update secret -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(prev, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(obj).Return(nil, fmt.Errorf("failure"))
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("update secret err -- %s", name), result)
	})
}

func TestSecretApplyGetErr(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, _, name, tearDown := setUpSecretHandler(t)
	defer tearDown()

	client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, fmt.Errorf("getErr"))
	client.EXPECT().Create(gomock.Any()).Times(0)
	client.EXPECT().Update(gomock.Any()).Times(0)
	client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	result := handler.Apply(rawData)
	assert.Equal(fmt.Sprintf("get secret err -- %s", name), result)
}

func TestSecretDelete(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpSecretHandler(t)
	defer tearDown()

	deletePolicy := metav1.DeletePropagationForeground

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(obj, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy}).Return(nil)

		result := handler.Delete(rawData)
		assert.Equal(fmt.Sprintf("delete secret -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(obj, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy}).Return(fmt.Errorf("failure"))

		result := handler.Delete(rawData)
		assert.Equal(fmt.Sprintf("delete secret err -- %s", name), result)
	})
}

func TestSecretDeleteError(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, _, name, tearDown := setUpSecretHandler(t)
	defer tearDown()

	errCases := []struct {
		name string
		err  error
		msg  string
	}{
		{name: "notfound", err: errors.NewNotFound(apiv1.Resource("secret"), name), msg: "secret does not exist"},
		{name: "othererr", err: fmt.Errorf("failure"), msg: "get secret err"},
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
