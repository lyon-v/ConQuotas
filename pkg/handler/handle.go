package handler

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	e "github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl/v2"
	"go.uber.org/zap"

	"RootfsQuota/pkg/config"
	"RootfsQuota/pkg/log"
	"RootfsQuota/pkg/xfs"
)

type RFSQuota struct {
	cfg           *config.Config
	stateManager  *xfs.StateManager
	projectIDPool *xfs.ProjectIDPool
	client        *containerd.Client
	ctx           context.Context
	cancel        context.CancelFunc
	sigCh         chan os.Signal
}

func NewRFSQuota(configPath string) (*RFSQuota, error) {
	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// 初始化状态管理器
	stateManager, err := xfs.NewStateManager(cfg.StateFilePath)
	if err != nil {
		return nil, err
	}

	// 初始化项目ID池
	projectIDPool := xfs.NewProjectIDPool(cfg.Project.IDMin, cfg.Project.IDMax)

	// 创建上下文和取消函数
	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认命名空间
	ctx = namespaces.WithNamespace(ctx, cfg.Namespace)

	return &RFSQuota{
		cfg:           cfg,
		stateManager:  stateManager,
		projectIDPool: projectIDPool,
		ctx:           ctx,
		cancel:        cancel,
		sigCh:         make(chan os.Signal, 1),
	}, nil
}

func (q *RFSQuota) Run() error {
	defer q.cleanup()
	signal.Notify(q.sigCh, syscall.SIGINT, syscall.SIGTERM)
	go q.handleSignals()

	// 主循环
	for {
		select {
		case <-q.ctx.Done():
			return nil
		default:
			if err := q.startEventListener(); err != nil {
				log.Error("Event listener failed, retrying", zap.Error(err))
				select {
				case <-time.After(5 * time.Second):
					continue
				case <-q.ctx.Done():
				}
				return nil
			}
		}
	}
}

func (q *RFSQuota) startEventListener() error {
	client, err := containerd.New(q.cfg.ContainerdSock, containerd.WithTimeout(10*time.Second))
	if err != nil {
		return err
	}
	q.client = client

	// 同步状态
	if err := q.syncState(); err != nil {
		log.Error("State sync failed", zap.Error(err))
	}

	// 订阅事件
	eventsCh, errCh := client.Subscribe(q.ctx)
	log.Info("Listening for containerd events...")

	for {
		select {
		case envelope := <-eventsCh:
			if err := q.handleEvent(envelope); err != nil {
				log.Error("Failed to handle event", zap.Error(err))
			}
		case err := <-errCh:
			return err
		case <-q.ctx.Done():
			return nil
		}
	}
}

func (q *RFSQuota) handleEvent(envelope *e.Envelope) error {
	event, err := typeurl.UnmarshalAny(envelope.Event)
	if err != nil {
		return err
	}

	switch e := event.(type) {
	case *events.TaskCreate:
		return q.handleTaskCreate(e)
	case *events.TaskDelete:
		return q.handleTaskDelete(e)
	}
	return nil
}

func (q *RFSQuota) handleTaskCreate(e *events.TaskCreate) error {
	upperdir := strings.TrimPrefix(e.Rootfs[0].Options[1], "upperdir=")

	projID, err := q.projectIDPool.Allocate()
	if err != nil {
		return err
	}

	if err := xfs.SetProjectIDWithXFSQuota(upperdir, projID); err != nil {
		q.projectIDPool.Release(projID)
		return err
	}

	if err := xfs.SetProjectQuotaWithXFSQuota(projID, q.cfg.Quota.DefaultSoft, q.cfg.Quota.DefaultSoft); err != nil {
		q.projectIDPool.Release(projID)
		return err
	}

	if err := q.stateManager.AddEntry(e.ContainerID, projID, upperdir); err != nil {
		q.projectIDPool.Release(projID)
		return err
	}

	log.Info("Quota set successfully",
		zap.String("container", e.ContainerID),
		zap.Uint32("projectID", projID))
	return nil
}

func (q *RFSQuota) handleTaskDelete(e *events.TaskDelete) error {
	upperdir, err := xfs.GetSnapshotUpperdir(q.ctx, q.client, e.ContainerID)
	if err != nil {
		return err
	}

	var projID uint32
	if _, err := os.Stat(upperdir); err == nil {
		projID, err = GetProjectID(e.ContainerID, upperdir, q.stateManager)
		if err != nil {
			return err
		}
	}

	if err := xfs.SetProjectQuotaWithXFSQuota(projID, "0", "0"); err != nil {
		return err
	}

	if err := q.stateManager.RemoveEntry(e.ContainerID); err != nil {
		return err
	}

	q.projectIDPool.Release(projID)
	log.Info("Quota removed successfully",
		zap.String("container", e.ContainerID),
		zap.Uint32("projectID", projID))
	return nil
}

func (q *RFSQuota) syncState() error {
	containers, err := q.client.Containers(q.ctx)
	if err != nil {
		return err
	}

	for _, c := range containers {
		id := c.ID()
		upperdir, err := xfs.GetSnapshotUpperdir(q.ctx, q.client, id)
		if err != nil {
			continue
		}

		if _, err := os.Stat(upperdir); os.IsNotExist(err) {
			continue
		}

		if _, exists := q.stateManager.GetEntry(id); !exists {
			if err := q.restoreQuota(id, upperdir); err != nil {
				log.Error("Failed to restore quota", zap.String("container", id), zap.Error(err))
			}
		}
	}
	return nil
}

func (q *RFSQuota) restoreQuota(containerID, upperdir string) error {
	projID, err := q.projectIDPool.Allocate()
	if err != nil {
		return err
	}

	if err := xfs.SetProjectIDWithXFSQuota(upperdir, projID); err != nil {
		q.projectIDPool.Release(projID)
		return err
	}

	if err := xfs.SetProjectQuotaWithXFSQuota(projID, q.cfg.Quota.DefaultSoft, q.cfg.Quota.DefaultHard); err != nil {
		q.projectIDPool.Release(projID)
		return err
	}

	return q.stateManager.AddEntry(containerID, projID, upperdir)
}

func (q *RFSQuota) handleSignals() {
	<-q.sigCh
	log.Info("Received shutdown signal")
	q.cancel()
}

func (q *RFSQuota) cleanup() {
	if q.client != nil {
		q.client.Close()
	}
	log.Sync()
}
