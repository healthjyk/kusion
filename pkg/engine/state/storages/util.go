package storages

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	stateFile  = "state.yaml"
	stateTable = "state"
)

// GenStateFilePath generates the state file path, which is used for LocalStorage.
func GenStateFilePath(dir, project, stack, workspace string) string {
	return filepath.Join(dir, project, stack, workspace, stateFile)
}

// GenGenericOssStateFileKey generates generic oss state file key, which is use for OssStorage and S3Storage.
func GenGenericOssStateFileKey(prefix, project, stack, workspace string) string {
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return fmt.Sprintf("%s%s/%s/%s/%s", prefix, project, stack, workspace, stateFile)
}
