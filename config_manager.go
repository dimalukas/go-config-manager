package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// ConfigManager encapsulates all logic for managing a single Viper configuration.
type ConfigManager[T any] struct {
	v               *viper.Viper
	config          *T
	mu              sync.RWMutex
	configPath      string
	backupPath      string
	requiredKeys    []string
	postRestoreFunc func()
}

// NewConfigManager creates and initializes a manager for a specific config file.
func NewConfigManager[T any](
	configPath, backupPath string,
	requiredKeys []string,
	configPtr *T,
	postRestoreFunc func(),
) (*ConfigManager[T], error) {
	if configPath == "" || backupPath == "" {
		return nil, fmt.Errorf("configPath and backupPath must not be empty")
	}

	manager := &ConfigManager[T]{
		v:               viper.New(),
		configPath:      configPath,
		backupPath:      backupPath,
		requiredKeys:    requiredKeys,
		config:          configPtr,
		postRestoreFunc: postRestoreFunc,
	}

	manager.v.SetConfigFile(configPath)
	manager.v.SetConfigType("yaml")

	if err := manager.v.ReadInConfig(); err != nil || !manager.isConfigValid() {
		if err := manager.restoreFromBackup(); err != nil {
			return nil, fmt.Errorf("error reading config '%s' and failed to restore from backup '%s': %w", configPath, manager.backupPath, err)
		}
	}

	if err := manager.v.Unmarshal(&manager.config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config from '%s': %w", configPath, err)
	}

	manager.v.WatchConfig()

	go manager.checkAndRestorePeriodically()

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
		log.Printf("config file '%s'is not valid YAML: %v", cm.configPath, err)
		return false
	}

	// Check all required keys are set
	required := make(map[string]struct{}, len(cm.requiredKeys))
	for _, key := range cm.requiredKeys {
		required[key] = struct{}{}
		if !cm.v.IsSet(key) {
			log.Printf("config file '%s' is missing required key: %s", cm.configPath, key)
			return false
		}
	}
	// Check for extra keys
	allKeys := cm.v.AllKeys()
	if len(allKeys) != len(cm.requiredKeys) {
		log.Printf("config file '%s' has extra or missing keys: %v (expected: %v)", cm.configPath, allKeys, cm.requiredKeys)
		return false
	}
	for _, key := range allKeys {
		if _, ok := required[key]; !ok {
			log.Printf("config file '%s' has unexpected extra key: %s", cm.configPath, key)
			return false
		}
	}
	// Try to unmarshal into the config struct to catch type errors
	var temp T
	return cm.v.Unmarshal(&temp) == nil
}

// restoreFromBackup reads the backup file and writes its content to the main config file for this manager.
func (cm *ConfigManager[T]) restoreFromBackup() error {
	backupData, err := os.ReadFile(cm.backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file '%s': %w", cm.backupPath, err)
	}
	if err := os.WriteFile(cm.configPath, backupData, 0644); err != nil {
		return fmt.Errorf("failed to write to main config file '%s': %w", cm.configPath, err)
	}
	cm.v = viper.New()
	cm.v.SetConfigFile(cm.configPath)
	cm.v.SetConfigType("yaml")
	if err := cm.v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read in config '%s' after restore: %w", cm.configPath, err)
	}
	if err := cm.v.Unmarshal(&cm.config); err != nil {
		return fmt.Errorf("failed to unmarshal '%s' after restore: %w", cm.configPath, err)
	}
	log.Printf("restored '%s' from backup: '%s'", cm.configPath, cm.backupPath)

	if cm.postRestoreFunc != nil {
		cm.postRestoreFunc()
	}

	return nil
}

// checkAndRestorePeriodically checks config health every 5 seconds.
func (cm *ConfigManager[T]) checkAndRestorePeriodically() {
	for {
		time.Sleep(5 * time.Second)
		if !cm.isConfigValid() {
			log.Printf("invalid config: %s", cm.configPath)
			cm.mu.Lock()
			if err := cm.restoreFromBackup(); err != nil {
				log.Printf("restore config '%s' error: %v", cm.configPath, err)
			}
			cm.mu.Unlock()
		}
	}
}

// printConfigPeriodically safely reads and prints the config every 3 seconds.
func (cm *ConfigManager[T]) printConfigPeriodically() {
	for {
		time.Sleep(3 * time.Second)
		cm.mu.RLock()
		log.Printf("%s: %+v\n", cm.configPath, *cm.config)
		cm.mu.RUnlock()
	}
}
