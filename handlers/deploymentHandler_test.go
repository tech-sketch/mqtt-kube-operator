package handlers

import (
	"fmt"
	"testing"

	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/tech-sketch/mqtt-kube-operator/mock"
)

func setUpDeploymentHandler(t *testing.T) (*deploymentHandler, *mock.MockDeploymentInterface, runtime.Object, *appsv1.Deployment, string, func()) {
	ctrl := gomock.NewController(t)

	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	logger, _ := loggerConfig.Build()

	clientset := mock.NewMockInterface(ctrl)
	mappsv1 := mock.NewMockAppsV1Interface(ctrl)
	deployment := mock.NewMockDeploymentInterface(ctrl)
	clientset.EXPECT().AppsV1().Return(mappsv1).AnyTimes()
	mappsv1.EXPECT().Deployments(gomock.Any()).Return(deployment).AnyTimes()

	handler := &deploymentHandler{
		kubeClient: clientset,
		logger:     logger.Sugar(),
	}

	_, rawData := getPayloadFromFixture(t, "../testdata/deployment.yaml")
	obj, ok := rawData.(*appsv1.Deployment)
	if !ok {
		t.Fatal(ok)
	}

	name := "my-deployment"

	return handler, deployment, rawData, obj, name, func() {
		logger.Sync()
		ctrl.Finish()
	}
}

func TestDeploymentCreate(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpDeploymentHandler(t)
	defer tearDown()

	getErr := errors.NewNotFound(appsv1.Resource("deployment"), name)

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, getErr)
		client.EXPECT().Create(obj).Return(obj, nil)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("create deployment -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, getErr)
		client.EXPECT().Create(obj).Return(nil, fmt.Errorf("failure"))
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("create deployment err -- %s", name), result)
	})
}

func TestDeploymentUpdate(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpDeploymentHandler(t)
	defer tearDown()

	prev := rawData.(*appsv1.Deployment)
	prev.ObjectMeta.Labels["test"] = "test"

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(prev, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(obj).Return(nil, nil)
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("update deployment -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(prev, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(obj).Return(nil, fmt.Errorf("failure"))
		client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

		result := handler.Apply(rawData)
		assert.Equal(fmt.Sprintf("update deployment err -- %s", name), result)
	})
}

func TestDeploymentApplyGetErr(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, _, name, tearDown := setUpDeploymentHandler(t)
	defer tearDown()

	client.EXPECT().Get(name, metav1.GetOptions{}).Return(nil, fmt.Errorf("getErr"))
	client.EXPECT().Create(gomock.Any()).Times(0)
	client.EXPECT().Update(gomock.Any()).Times(0)
	client.EXPECT().Delete(gomock.Any(), gomock.Any()).Times(0)

	result := handler.Apply(rawData)
	assert.Equal(fmt.Sprintf("get deployment err -- %s", name), result)
}

func TestDeploymentDelete(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, obj, name, tearDown := setUpDeploymentHandler(t)
	defer tearDown()

	deletePolicy := metav1.DeletePropagationForeground

	t.Run("success", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(obj, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy}).Return(nil)

		result := handler.Delete(rawData)
		assert.Equal(fmt.Sprintf("delete deployment -- %s", name), result)
	})
	t.Run("failure", func(t *testing.T) {
		client.EXPECT().Get(name, metav1.GetOptions{}).Return(obj, nil)
		client.EXPECT().Create(gomock.Any()).Times(0)
		client.EXPECT().Update(gomock.Any()).Times(0)
		client.EXPECT().Delete(name, &metav1.DeleteOptions{PropagationPolicy: &deletePolicy}).Return(fmt.Errorf("failure"))

		result := handler.Delete(rawData)
		assert.Equal(fmt.Sprintf("delete deployment err -- %s", name), result)
	})
}

func TestDeploymentDeleteError(t *testing.T) {
	assert := assert.New(t)
	handler, client, rawData, _, name, tearDown := setUpDeploymentHandler(t)
	defer tearDown()

	errCases := []struct {
		name string
		err  error
		msg  string
	}{
		{name: "notfound", err: errors.NewNotFound(appsv1.Resource("deployment"), name), msg: "deployment does not exist"},
		{name: "othererr", err: fmt.Errorf("failure"), msg: "get deployment err"},
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
