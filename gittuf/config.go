package gittuf

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	tufdata "github.com/theupdateframework/go-tuf/data"
)

var ConfigPaths = []string{
	".gittufconfig.json",
	".config/gittuf/config.json",
}

type GitTUFConfigFile struct {
	SigningKey string
}

type GitTUFConfig struct {
	PrivateKey tufdata.PrivateKey
}

func FindConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	for _, p := range ConfigPaths {
		fullPath := path.Join(homeDir, p)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}
	return "", fmt.Errorf("gittuf config file not found")
}

func ReadConfig(path string) (*GitTUFConfig, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return &GitTUFConfig{}, err
	}

	var c GitTUFConfigFile
	err = json.Unmarshal(contents, &c)
	if err != nil {
		return &GitTUFConfig{}, err
	}

	key, err := LoadEd25519PrivateKeyFromSslib(c.SigningKey)
	if err != nil {
		return &GitTUFConfig{}, err
	}

	return &GitTUFConfig{PrivateKey: key}, nil
}
