package goconfigmanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// ConfigManager encapsulates all logic for managing a single Viper configuration.
type ConfigManager[T any] struct {
	v               *viper.Viper
	Config          *T
	mu              sync.RWMutex
	ConfigPath      string
	BackupPath      string
	requiredKeys    []string
	checkInterval   time.Duration
	logInterval     time.Duration
	watchCtx        context.Context
	watchCancel     context.CancelFunc
	preRestoreFunc  func()
	postRestoreFunc func()
}

// NewConfigManager creates and initializes a manager for a specific config file.
func NewConfigManager[T any](
	configPath, backupPath string,
	requiredKeys []string,
	configPtr *T,
	checkInterval time.Duration,
	logInterval time.Duration,
	preRestoreFunc func(),
	postRestoreFunc func(),
) (*ConfigManager[T], error) {
	if configPath == "" {
		return nil, fmt.Errorf("configPath must not be empty")
	}

	manager := &ConfigManager[T]{
		v:               viper.New(),
		ConfigPath:      configPath,
		BackupPath:      backupPath,
		requiredKeys:    requiredKeys,
		Config:          configPtr,
		checkInterval:   checkInterval,
		logInterval:     logInterval,
		preRestoreFunc:  preRestoreFunc,
		postRestoreFunc: postRestoreFunc,
	}

	manager.v.SetConfigFile(configPath)
	manager.v.SetConfigType("yaml")

	if err := manager.v.ReadInConfig(); err != nil || !manager.isConfigValid() {
		if err := manager.restoreFromBackup(); err != nil {
			return nil, fmt.Errorf("error reading config '%s' and failed to restore from backup '%s': %w", configPath, manager.BackupPath, err)
		}
	}

	if err := manager.v.Unmarshal(&manager.Config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config from '%s': %w", configPath, err)
	}

	manager.v.WatchConfig()
	manager.v.OnConfigChange(func(e fsnotify.Event) {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		if manager.isConfigValid() {
			if err := manager.v.Unmarshal(manager.Config); err != nil {
				log.Printf("failed to unmarshal config on change: %v", err)
			}
		}
	})

	log.Printf("Config loaded from '%s'", configPath)
	return manager, nil
}

// isConfigValid checks if all required keys are present and valid in this manager's viper instance.
func (cm *ConfigManager[T]) isConfigValid() bool {
	if cm.v == nil {
		return false
	}
	// Try to re-read the config file
	if err := cm.v.ReadInConfig(); err != nil {
		log.Printf("config file '%s'is not valid YAML: %v", cm.ConfigPath, err)
		return false
	}

	if len(cm.requiredKeys) == 0 {
		return true
	}

	// Check all required keys are set
	required := make(map[string]struct{}, len(cm.requiredKeys))
	for _, key := range cm.requiredKeys {
		required[key] = struct{}{}
		if !cm.v.IsSet(key) {
			log.Printf("config file '%s' is missing required key: %s", cm.ConfigPath, key)
			return false
		}
	}
	// Check for extra keys
	allKeys := cm.v.AllKeys()
	if len(allKeys) != len(cm.requiredKeys) {
		log.Printf("config file '%s' has extra or missing keys: %v (expected: %v)", cm.ConfigPath, allKeys, cm.requiredKeys)
		return false
	}
	for _, key := range allKeys {
		if _, ok := required[key]; !ok {
			log.Printf("config file '%s' has unexpected extra key: %s", cm.ConfigPath, key)
			return false
		}
	}
	// Try to unmarshal into the config struct to catch type errors
	var temp T
	return cm.v.Unmarshal(&temp) == nil
}

// restoreFromBackup reads the backup file and writes its content to the main config file for this manager.
func (cm *ConfigManager[T]) restoreFromBackup() error {
	backupData, err := os.ReadFile(cm.BackupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file '%s': %w", cm.BackupPath, err)
	}
	if err := os.WriteFile(cm.ConfigPath, backupData, 0644); err != nil {
		return fmt.Errorf("failed to write to main config file '%s': %w", cm.ConfigPath, err)
	}

	if cm.preRestoreFunc != nil {
		cm.preRestoreFunc()
	}

	cm.v = viper.New()
	cm.v.SetConfigFile(cm.ConfigPath)
	cm.v.SetConfigType("yaml")
	if err := cm.v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read in config '%s' after restore: %w", cm.ConfigPath, err)
	}
	if err := cm.v.Unmarshal(&cm.Config); err != nil {
		return fmt.Errorf("failed to unmarshal '%s' after restore: %w", cm.ConfigPath, err)
	}
	log.Printf("restored '%s' from backup: '%s'", cm.ConfigPath, cm.BackupPath)

	if cm.postRestoreFunc != nil {
		cm.postRestoreFunc()
	}

	return nil
}

// StartWatch starts the periodic config check in a background goroutine.
// If already running, it stops the previous watcher before starting a new one.
func (cm *ConfigManager[T]) StartWatch() {
	cm.StopWatch() // Stop any existing watcher
	cm.watchCtx, cm.watchCancel = context.WithCancel(context.Background())
	go cm.watchLoop(cm.watchCtx)
}

// StopWatch stops the periodic config check if running.
func (cm *ConfigManager[T]) StopWatch() {
	if cm.watchCancel != nil {
		cm.watchCancel()
		cm.watchCancel = nil
	}
}

// watchLoop periodically checks the config file's validity at the configured interval.
// If the config is invalid, it attempts to restore from backup. The check stops when the context is cancelled.
func (cm *ConfigManager[T]) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(cm.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("Stopped periodic config check for %s", cm.ConfigPath)
			return
		case <-ticker.C:
			if !cm.isConfigValid() {
				log.Printf("invalid config: %s", cm.ConfigPath)
				cm.mu.Lock()
				if err := cm.restoreFromBackup(); err != nil {
					log.Printf("restore config '%s' error: %v", cm.ConfigPath, err)
				}
				cm.mu.Unlock()
			}
		}
	}
}

// LogConfig periodically logs the current config at the configured logInterval.
func (cm *ConfigManager[T]) LogConfig() {
	for {
		time.Sleep(cm.logInterval)
		cm.mu.RLock()
		log.Printf("%s: %+v\n", cm.ConfigPath, *cm.Config)
		cm.mu.RUnlock()
	}
}

// SetKey sets a key-value pair in the Viper config.
func (cm *ConfigManager[T]) SetKey(key string, value interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.v.Set(key, value)
}

// SaveConfig writes the config to file. If unmarshal is true, also updates the struct.
func (cm *ConfigManager[T]) SaveConfig(unmarshal bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if err := cm.v.WriteConfig(); err != nil {
		return err
	}
	if unmarshal {
		if err := cm.v.Unmarshal(&cm.Config); err != nil {
			return err
		}
	}
	return nil
}
