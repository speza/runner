package docker

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/speza/runner/pkg"
	"io"
	"os"
)

type Runner struct {
	Client client.APIClient
}

func (r Runner) Provision(ctx context.Context, uuid string, task pkg.TaskSpecification) (pkg.ServerAddress, error) {
	if !task.ImageLocal {
		reader, err := r.Client.ImagePull(ctx, task.Image, types.ImagePullOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to pull image: %w", err)
		}
		io.Copy(os.Stdout, reader)
	}

	output, err := r.Client.ContainerCreate(ctx, &container.Config{
		Image:        task.Image,
		AttachStderr: true,
		AttachStdout: true,
	}, &container.HostConfig{
		PublishAllPorts: true,
	}, &network.NetworkingConfig{}, uuid)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	if err = r.Client.ContainerStart(ctx, output.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	go func() {
		reader, err := r.Client.ContainerLogs(context.Background(), output.ID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		})
		if err != nil {
			log.WithError(err).Error("failed to tail logs")
		}
		defer reader.Close()

		f, err := os.Create(fmt.Sprintf("log-%s.txt", uuid))
		if err != nil {
			log.WithError(err).Error("failed to create log file")
		}
		defer f.Close()

		scanner := bufio.NewScanner(reader)
		writer := bufio.NewWriter(f)
		for scanner.Scan() {
			writer.WriteString(scanner.Text() + "\n")
			writer.Flush()
		}
	}()

	inspect, err := r.Client.ContainerInspect(ctx, output.ID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	grpcClientPort := inspect.NetworkSettings.Ports["5300/tcp"][0]
	serverAddr := fmt.Sprintf("%s:%s", grpcClientPort.HostIP, grpcClientPort.HostPort)
	return pkg.ServerAddress(serverAddr), nil
}

func (r Runner) Teardown(ctx context.Context, uuid string) error {
	if err := r.Client.ContainerRemove(ctx, uuid, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("error removing container: %w", err)
	}
	return nil
}
