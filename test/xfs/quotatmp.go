package xfs

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/containerd/containerd"
)

// GetProjectIDFromXFS retrieves the XFS project ID for a given file path.
func GetProjectIDFromXFS(path string) (uint32, error) {
	cmd := exec.Command("xfs_io", "-r", "-c", "stat", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("执行 xfs_io 失败: %v, 输出: %s", err, string(output))
	}

	// 使用正则表达式匹配 projid
	re := regexp.MustCompile(`projid\s*=\s*(\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return 0, fmt.Errorf("未找到 projid 信息")
	}

	// 转换为数字
	projid, err := strconv.ParseUint(matches[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("projid 转换失败: %v", err)
	}

	return uint32(projid), nil
}

// SetProjectIDWithXFSQuota sets an XFS project ID for a given path using xfs_quota.
func SetProjectIDWithXFSQuota(path string, projid uint32) error {
	// Format the xfs_quota command: xfs_quota -x -c "project -s -p <path> <projid>"
	cmdStr := fmt.Sprintf("project -s -p %s %d", path, projid)
	cmd := exec.Command("xfs_quota", "-x", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行 xfs_quota 失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

// SetProjectQuotaWithXFSQuota sets XFS project quota limits for a given path and project ID.
// It executes the xfs_quota command to set soft and hard block limits to the specified values.
func SetProjectQuotaWithXFSQuota(projid uint32, bsoft, bhard string) error {
	// Format the xfs_quota command: xfs_quota -x -c "limit -p bsoft=<bsoft> bhard=<bhard> <projid>"
	cmdStr := fmt.Sprintf("limit -p bsoft=%s bhard=%s %d", bsoft, bhard, projid)
	cmd := exec.Command("xfs_quota", "-x", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute xfs_quota: %v, output: %s", err, string(output))
	}
	return nil
}

// getSnapshotUpperdir retrieves the upperdir path for a container's snapshot using the containerd API.
func GetSnapshotUpperdir(ctx context.Context, client *containerd.Client, containerID string) (string, error) {
	// Ensure the context has the correct namespace
	container, err := client.LoadContainer(ctx, containerID)
	if err != nil {
		log.Printf("加载容器失败: %v", err)
		return "", err
	}
	// 获取容器信息
	info, err := container.Info(ctx)
	if err != nil {
		log.Printf("获取容器信息失败: %v", err)
		return "", err
	}
	// 获取快照器和快照键
	snapshotterName := info.Snapshotter
	snapshotKey := info.SnapshotKey
	snapshotter := client.SnapshotService(snapshotterName)
	// 获取快照的挂载信息
	mounts, err := snapshotter.Mounts(ctx, snapshotKey)
	if err != nil {
		log.Printf("获取快照挂载信息失败: %v", err)
		return "", err
	}
	// 解析挂载选项以获取upperdir和lowerdir
	var upperDir string
	var lowerDir string
	for _, mount := range mounts {

		if mount.Type == "overlay" {
			for _, opt := range mount.Options {
				if strings.HasPrefix(opt, "upperdir=") {
					upperDir = strings.TrimPrefix(opt, "upperdir=")
					// fmt.Printf("upperdir: %s\n", upperDir)
				} else if strings.HasPrefix(opt, "lowerdir=") {
					lowerDir = strings.TrimPrefix(opt, "lowerdir=")
					fmt.Printf("lowerdir: %s\n", lowerDir)
				}
			}
		}
	}
	return upperDir, nil
}
