package main

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"github.com/speza/runner/pkg"
	"github.com/speza/runner/pkg/docker"
	"github.com/speza/runner/proto"
	"google.golang.org/grpc"
	"net/http"
)

type Scheduler struct {
	scheduledTasks chan pkg.TaskSpecification
	executors      int
	runner         pkg.Runner
}

func (s *Scheduler) work(workerID int) {
	for {
		task := <-s.scheduledTasks

		ctx := context.Background()
		id := uuid.New().String()
		logger := log.WithField("worker", workerID).WithField("container", id)

		logger.Info("task start")

		serverAddr, err := s.runner.Provision(ctx, id, task)
		if err != nil {
			logger.WithError(err).Error("failed to provision runner")
			continue
		}

		conn, err := grpc.Dial(string(serverAddr), grpc.WithInsecure())
		if err != nil {
			logger.WithError(err).Error("failed to dial grpc")
			s.teardown(ctx, id, logger)
			continue
		}
		executor := proto.NewExecutorClient(conn)
		execute, err := executor.Do(ctx, &proto.Request{
			Name: task.Name,
			Args: task.Args,
		})
		if err != nil {
			logger.WithError(err).Error("failed to execute")
			s.teardown(ctx, id, logger)
			continue
		}
		logger.Info(execute.Message)

		s.teardown(ctx, id, logger)

		logger.Info("task finished")
	}
}

func (s *Scheduler) teardown(ctx context.Context, id string, logger *log.Entry) {
	if err := s.runner.Teardown(ctx, id); err != nil {
		logger.WithError(err).Error("failed to teardown runner")
	}
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	dc, err := client.NewEnvClient()
	if err != nil {
		log.WithError(err).Fatal("failed to create Docker client")
	}

	runner := docker.Runner{
		Client: dc,
	}

	scheduler := &Scheduler{
		runner:         runner,
		scheduledTasks: make(chan pkg.TaskSpecification, 10),
		executors:      2,
	}

	e.POST("/task", func(c echo.Context) error {
		spec := pkg.TaskSpecification{}
		if err := c.Bind(&spec); err != nil {
			return c.String(http.StatusInternalServerError, "error")
		}
		scheduler.scheduledTasks <- spec
		return c.String(http.StatusOK, "success")
	})

	for i := 0; i < scheduler.executors; i++ {
		go scheduler.work(i)
	}

	e.Logger.Fatal(e.Start(":8000"))
}
