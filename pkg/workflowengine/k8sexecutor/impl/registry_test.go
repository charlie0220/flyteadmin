package impl

import (
	"testing"

	"github.com/flyteorg/flyteadmin/pkg/workflowengine/k8sexecutor/interfaces"
	"github.com/flyteorg/flyteadmin/pkg/workflowengine/k8sexecutor/mocks"
	"github.com/stretchr/testify/assert"
)

func getMockK8sWorkflowExecutor(id string) interfaces.K8sWorkflowExecutor {
	exec := mocks.K8sWorkflowExecutor{}
	exec.OnID().Return(id)
	return &exec
}

var testExecID = "foo"
var defaultExecID = "default"

func TestRegister(t *testing.T) {
	registry := flyteK8sWorkflowExecutorRegistry{}
	exec := getMockK8sWorkflowExecutor(testExecID)
	registry.Register(exec)
	assert.Equal(t, testExecID, registry.GetExecutor().ID())
}

func TestRegisterDefault(t *testing.T) {
	registry := flyteK8sWorkflowExecutorRegistry{}

	defaultExec := getMockK8sWorkflowExecutor(defaultExecID)
	registry.RegisterDefault(defaultExec)
	assert.Equal(t, defaultExecID, registry.GetExecutor().ID())

	exec := getMockK8sWorkflowExecutor(testExecID)
	registry.Register(exec)
	assert.Equal(t, testExecID, registry.GetExecutor().ID())
}
