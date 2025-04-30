package config

import (
	"encoding/json"
	"fmt"
	"os"

	"RootfsQuota/pkg/log"

	"go.uber.org/zap"
)

// Config 存储配置文件中的参数，部分字段采用嵌套结构
type Config struct {
	StateFilePath  string        `json:"state_file_path"`
	Project        ProjectConfig `json:"project"`
	MetricsPort    string        `json:"metrics_port"`
	ContainerdSock string        `json:"containerd_sock"`
	Quota          QuotaConfig   `json:"quota"`
	Namespace      string        `json:"namespace"`
}

// ProjectConfig 存储项目 ID 范围相关配置
type ProjectConfig struct {
	IDMin uint32 `json:"id_min"`
	IDMax uint32 `json:"id_max"`
}

// QuotaConfig 存储默认配额相关配置
type QuotaConfig struct {
	DefaultSoft string `json:"default_soft"`
	DefaultHard string `json:"default_hard"`
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
	if cfg.Project.IDMin == 0 || cfg.Project.IDMax == 0 || cfg.Project.IDMin >= cfg.Project.IDMax {
		return nil, fmt.Errorf("invalid project.id range: min=%d, max=%d", cfg.Project.IDMin, cfg.Project.IDMax)
	}
	if cfg.ContainerdSock == "" {
		return nil, fmt.Errorf("containerd_sock is required")
	}

	// 处理可为空字段
	if cfg.MetricsPort == "" {
		cfg.MetricsPort = "" // 明确设置为空，表示禁用监控或后续逻辑处理
	}
	if cfg.Quota.DefaultSoft == "" {
		cfg.Quota.DefaultSoft = "10g" // 允许为空，后续逻辑可处理
	}
	if cfg.Quota.DefaultHard == "" {
		cfg.Quota.DefaultHard = "10g" // 允许为空，后续逻辑可处理
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "default" // 设置默认值
	}

	log.Info("Loaded configuration", zap.String("file", filePath))
	return &cfg, nil
}
