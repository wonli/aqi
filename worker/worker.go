package worker

import (
	"fmt"
	"os"

	"github.com/hibiken/asynq"

	"github.com/wonli/aqi/logger"
)

var Engine *EngineClient

type EngineClient struct {
	Running bool
	Router  map[string]Task

	Opt    *asynq.RedisClientOpt
	Server *asynq.Server
}

func InitEngine(rds *asynq.RedisClientOpt, config asynq.Config) *EngineClient {
	server := asynq.NewServer(rds, config)
	Engine = &EngineClient{
		Opt:    rds,
		Server: server,
		Router: map[string]Task{},
	}

	return Engine
}

func (e *EngineClient) Register(t Task) {
	if e.Running {
		logger.SugarLog.Errorf("please register in router")
		return
	}

	name := t.GetName()
	if name == "" {
		logger.SugarLog.Errorf("failed to register, name is empty")
		return
	}

	e.Router[name] = t
}

func (e *EngineClient) Add(task *asynq.Task) error {
	t := task.Type()
	if t == "" {
		return fmt.Errorf("task type is undefined")
	}

	_, ok := e.Router[t]
	if !ok {
		return fmt.Errorf("task not registered")
	}

	client := asynq.NewClient(e.Opt)
	defer client.Close()

	_, err := client.Enqueue(task)
	if err != nil {
		return err
	}

	return nil
}

func (e *EngineClient) Run() {
	s := asynq.NewServeMux()
	for name, handler := range e.Router {
		s.Handle(name, handler)
	}

	e.Running = true
	err := e.Server.Run(s)
	if err != nil {
		logger.SugarLog.Errorf("failed to start asynq service: :%s", err.Error())
		os.Exit(0)
	}
}
