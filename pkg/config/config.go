package config

import (
	"encoding/json"
	"fmt"
	"os"

	"RootfsQuota/pkg/log"

	"go.uber.org/zap"
)

// Config 存储配置文件中的参数
type Config struct {
	StateFilePath    string `json:"state_file_path"`
	ProjectIDMin     uint32 `json:"project_id_min"`
	ProjectIDMax     uint32 `json:"project_id_max"`
	MetricsPort      string `json:"metrics_port"`
	ContainerdSock   string `json:"containerd_sock"`
	DefaultQuotaSoft string `json:"default_quota_soft"`
	DefaultQuotaHard string `json:"default_quota_hard"`
	Namespace        string `json:"namespace"`
}

// LoadConfig 从指定路径加载配置文件
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// 验证必填字段
	if cfg.StateFilePath == "" {
		return nil, fmt.Errorf("state_file_path is required")
	}
	if cfg.ProjectIDMin == 0 || cfg.ProjectIDMax == 0 || cfg.ProjectIDMin >= cfg.ProjectIDMax {
		return nil, fmt.Errorf("invalid project_id range: min=%d, max=%d", cfg.ProjectIDMin, cfg.ProjectIDMax)
	}
	if cfg.MetricsPort == "" {
		return nil, fmt.Errorf("metrics_port is required")
	}
	if cfg.ContainerdSock == "" {
		return nil, fmt.Errorf("containerd_sock is required")
	}
	if cfg.DefaultQuotaSoft == "" || cfg.DefaultQuotaHard == "" {
		return nil, fmt.Errorf("default_quota_soft and default_quota_hard are required")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}

	log.Info("Loaded configuration", zap.String("file", filePath))
	return &cfg, nil
}
