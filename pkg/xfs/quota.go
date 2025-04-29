package xfs

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"RootfsQuota/pkg/log"

	"github.com/containerd/containerd"
	"go.uber.org/zap"
)

// GetProjectIDFromXFS retrieves the XFS project ID for a given file path.
func GetProjectIDFromXFS(path string) (uint32, error) {
	cmd := exec.Command("xfs_io", "-r", "-c", "stat", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to execute xfs_io: %v, output: %s", err, string(output))
	}

	re := regexp.MustCompile(`projid\s*=\s*(\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return 0, fmt.Errorf("projid not found")
	}

	projid, err := strconv.ParseUint(matches[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse projid: %v", err)
	}

	return uint32(projid), nil
}

// SetProjectIDWithXFSQuota sets an XFS project ID for a given path using xfs_quota.
func SetProjectIDWithXFSQuota(path string, projid uint32) error {
	cmdStr := fmt.Sprintf("project -s -p %s %d", path, projid)
	cmd := exec.Command("xfs_quota", "-x", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute xfs_quota: %v, output: %s", err, string(output))
	}
	return nil
}

// SetProjectQuotaWithXFSQuota sets XFS project quota limits for a given project ID.
func SetProjectQuotaWithXFSQuota(projid uint32, bsoft, bhard string) error {
	cmdStr := fmt.Sprintf("limit -p bsoft=%s bhard=%s %d", bsoft, bhard, projid)
	cmd := exec.Command("xfs_quota", "-x", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute xfs_quota: %v, output: %s", err, string(output))
	}
	return nil
}

// GetSnapshotUpperdir retrieves the upperdir path for a container's snapshot.
func GetSnapshotUpperdir(ctx context.Context, client *containerd.Client, containerID string) (string, error) {
	container, err := client.LoadContainer(ctx, containerID)
	if err != nil {
		log.Error("Failed to load container", zap.String("containerID", containerID), zap.Error(err))
		return "", err
	}

	info, err := container.Info(ctx)
	if err != nil {
		log.Error("Failed to get container info", zap.String("containerID", containerID), zap.Error(err))
		return "", err
	}

	snapshotterName := info.Snapshotter
	snapshotKey := info.SnapshotKey
	snapshotter := client.SnapshotService(snapshotterName)

	mounts, err := snapshotter.Mounts(ctx, snapshotKey)
	if err != nil {
		log.Error("Failed to get snapshot mounts", zap.String("containerID", containerID), zap.Error(err))
		return "", err
	}

	var upperDir string
	for _, mount := range mounts {
		if mount.Type == "overlay" {
			for _, opt := range mount.Options {
				if strings.HasPrefix(opt, "upperdir=") {
					upperDir = strings.TrimPrefix(opt, "upperdir=")
				}
			}
		}
	}
	if upperDir == "" {
		return "", fmt.Errorf("upperdir not found for container %s", containerID)
	}
	return upperDir, nil
}
