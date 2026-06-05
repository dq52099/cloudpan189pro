package bootstrap

import (
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/taskengine"
	"go.uber.org/zap"
)

type mockCloseTaskEngine struct {
	taskengine.TaskEngine

	running bool
	stopped bool
}

func (m *mockCloseTaskEngine) IsRunning() bool {
	return m.running
}

func (m *mockCloseTaskEngine) Stop() error {
	m.stopped = true

	return nil
}

func TestServiceContextCloseTreatsTypedNilTaskEngineAsMissing(t *testing.T) {
	var taskEngine *mockCloseTaskEngine

	svc := &serviceContext{
		logger:     zap.NewNop(),
		taskEngine: taskEngine,
	}

	svc.Close()
}
