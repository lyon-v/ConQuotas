package handler

import (
	"fmt"

	"go.uber.org/zap"

	"RootfsQuota/pkg/log"
	"RootfsQuota/pkg/xfs"
)

func GetProjectID(containerId, path string, stateManager *xfs.StateManager) (uint32, error) {

	id, err := xfs.GetProjectIDFromXFS(path)

	log.Info("File exists", zap.Uint32("projectID", id))

	entry, exists := stateManager.GetEntry(containerId)
	if !exists && err != nil {
		return id, fmt.Errorf("failed get projid from stateManager: %v", containerId)
	}
	if id != 0 && entry.ProjectID != id && exists {
		return id, fmt.Errorf("containereId %v, wrong projied-> xfs: %v, stateManager: %v", containerId, id, entry.ProjectID)
	}
	return id, nil

}
