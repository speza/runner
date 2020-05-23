package pkg

import (
	"context"
)

type TaskSpecification struct {
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	ImageLocal bool              `json:"image_local"`
	Args       map[string]string `json:"args"`
}

type ServerAddress string

type Runner interface {
	Provision(ctx context.Context, uuid string, task TaskSpecification) (ServerAddress, error)
	Teardown(ctx context.Context, uuid string) error
}
