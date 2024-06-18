package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

type Task interface {
	GetName() string
	ProcessTask(ctx context.Context, task *asynq.Task) error
}
