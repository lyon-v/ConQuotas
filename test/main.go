package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl/v2"
	"go.uber.org/zap"

	"RootfsQuota/pkg/config"
	"RootfsQuota/pkg/handler"
	"RootfsQuota/pkg/log"
	"RootfsQuota/pkg/xfs"
)

func main() {

	log.Info("RootfsQuota is starting...")
	// 解析命令行参数
	configPath := flag.String("config", "/etc/containerd-quota/config.json", "Path to configuration file")
	flag.Parse()

	// 加载配置文件
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Error("Failed to load configuration", zap.Error(err))
		os.Exit(1)
	}
	log.Info("RootfsQuota load config succeed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer log.Sync()

	// 初始化状态管理器
	stateManager, err := xfs.NewStateManager(cfg.StateFilePath)
	if err != nil {
		log.Error("Failed to initialize state manager", zap.Error(err))
		os.Exit(1)
	}
	log.Info("StateManager init succeed")
	// 初始化项目 ID 池
	projectIDPool := xfs.NewProjectIDPool(cfg.ProjectIDMin, cfg.ProjectIDMax)
	log.Info("ProjectIDPool init succeed")

	// 处理信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Info("Received shutdown signal, shutting down")
		cancel()
	}()

	// 默认命名空间
	ctx = namespaces.WithNamespace(ctx, cfg.Namespace)

	// 主循环，处理重连
	for {
		if err := runEventListener(ctx, stateManager, projectIDPool, cfg); err != nil {
			log.Error("Event listener failed, retrying in 5 seconds", zap.Error(err))
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				log.Info("Shutting down event listener")
				return
			}
		}
	}
}

func runEventListener(ctx context.Context, stateManager *xfs.StateManager, projectIDPool *xfs.ProjectIDPool, cfg *config.Config) error {
	client, err := containerd.New(cfg.ContainerdSock, containerd.WithTimeout(10*time.Second))
	if err != nil {
		return err
	}
	defer client.Close()

	// 启动时同步状态
	if err := syncState(ctx, client, stateManager, projectIDPool, cfg); err != nil {
		log.Error("State sync failed", zap.Error(err))
	}

	// 订阅事件
	eventsCh, errCh := client.Subscribe(ctx)
	log.Info("Connected to containerd, listening for events")

	for {
		select {
		case envelope := <-eventsCh:
			event, err := typeurl.UnmarshalAny(envelope.Event)
			if err != nil {
				log.Error("Failed to unmarshal event", zap.Error(err))
				continue
			}

			switch e := event.(type) {
			case *events.TaskCreate:
				upperdir := strings.TrimPrefix(e.Rootfs[0].Options[1], "upperdir=")
				projID, err := projectIDPool.Allocate()
				if err != nil {
					log.Error("Failed to allocate project ID", zap.Error(err))
					continue
				}
				if err := xfs.SetProjectIDWithXFSQuota(upperdir, projID); err != nil {
					log.Error("SetProjectID failed", zap.String("upperdir", upperdir), zap.Error(err))
					projectIDPool.Release(projID)
					continue
				}
				if err := xfs.SetProjectQuotaWithXFSQuota(projID, cfg.DefaultQuotaSoft, cfg.DefaultQuotaHard); err != nil {
					log.Error("SetProjectQuota failed", zap.String("upperdir", upperdir), zap.Error(err))
					projectIDPool.Release(projID)
					continue
				}
				if err := stateManager.AddEntry(e.ContainerID, projID, upperdir); err != nil {
					log.Error("Failed to save state", zap.String("containerID", e.ContainerID), zap.Error(err))
				}
				log.Info("SetQuota completed",
					zap.String("upperdir", upperdir),
					zap.Uint32("projectID", projID))

			case *events.TaskDelete:
				updir, err := xfs.GetSnapshotUpperdir(ctx, client, e.ContainerID)
				if err != nil {
					log.Error("GetSnapshotUpperdir failed", zap.String("containerID", e.ContainerID), zap.Error(err))
					continue
				}
				log.Info("TaskDelete", zap.String("upperdir", updir))

				var id uint32
				if _, err := os.Stat(updir); err == nil {

					id, err = handler.GetProjectID(e.ContainerID, updir, stateManager)
					if err != nil {
						log.Error("Failed get projectId", zap.String("containerID", e.ContainerID), zap.Error(err))
						continue
					}

				} else if os.IsNotExist(err) {
					log.Info("File does not exist", zap.String("upperdir", updir))
				} else {
					log.Error("Stat error", zap.String("upperdir", updir), zap.Error(err))
				}

				if err := xfs.SetProjectQuotaWithXFSQuota(id, "0", "0"); err != nil {
					log.Error("SetProjectQuota zero failed", zap.String("upperdir", updir), zap.Error(err))
				}
				if err := stateManager.RemoveEntry(e.ContainerID); err != nil {
					log.Error("Failed to remove state", zap.String("containerID", e.ContainerID), zap.Error(err))
				}
				projectIDPool.Release(id)
				log.Info("DeleteQuota completed",
					zap.String("upperdir", updir),
					zap.Uint32("projectID", id))

				// default:
				// 	fmt.Println(e)
				// 	log.Info("Event Catched", zap.String("event", "unkown"))

			}

		case err := <-errCh:
			return err

		case <-ctx.Done():
			return nil
		}
	}
}

func syncState(ctx context.Context, client *containerd.Client, stateManager *xfs.StateManager, projectIDPool *xfs.ProjectIDPool, cfg *config.Config) error {
	containers, err := client.Containers(ctx)
	if err != nil {
		return err
	}

	for _, c := range containers {
		id := c.ID()
		upperdir, err := xfs.GetSnapshotUpperdir(ctx, client, id)
		if err != nil {
			log.Error("GetSnapshotUpperdir failed", zap.String("containerID", id), zap.Error(err))
			continue
		}
		if _, err := os.Stat(upperdir); os.IsNotExist(err) {
			continue
		}

		entry, exists := stateManager.GetEntry(id)
		if !exists {
			projID, err := projectIDPool.Allocate()
			if err != nil {
				log.Error("Failed to allocate project ID", zap.Error(err))
				continue
			}
			if err := xfs.SetProjectIDWithXFSQuota(upperdir, projID); err != nil {
				log.Error("SetProjectID failed", zap.String("upperdir", upperdir), zap.Error(err))
				projectIDPool.Release(projID)
				continue
			}
			if err := xfs.SetProjectQuotaWithXFSQuota(projID, cfg.DefaultQuotaSoft, cfg.DefaultQuotaHard); err != nil {
				log.Error("SetProjectQuota failed", zap.String("upperdir", upperdir), zap.Error(err))
				projectIDPool.Release(projID)
				continue
			}
			if err := stateManager.AddEntry(id, projID, upperdir); err != nil {
				log.Error("Failed to save state", zap.String("containerID", id), zap.Error(err))
			}
			log.Info("Restored quota",
				zap.String("containerID", id),
				zap.String("upperdir", upperdir),
				zap.Uint32("projectID", projID))
		} else {
			projectIDPool.MarkUsed(entry.ProjectID)
		}
	}
	return nil
}
