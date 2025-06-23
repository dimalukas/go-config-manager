package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

// Config represents the structure of our YAML file.
type Config struct {
	Port        int `mapstructure:"port"`
	UpdateCount int `mapstructure:"updatecount"`
}

var (
	config *Config
)

func main() {
	config = &Config{}

	mainConfigKeys := []string{
		"port", "updatecount",
	}

	mainConfigManager, err := NewConfigManager(
		"config.yaml", "config.backup.yaml",
		mainConfigKeys,
		config,
		5*time.Second,
		3*time.Second,
		postRestoreFunc,
	)
	if err != nil {
		log.Fatalf("init failed: %v", err)
	}

	go simulateConfigChanges(mainConfigManager)
	go mainConfigManager.addRandomKeyPeriodically()
	go mainConfigManager.corruptConfigFilePeriodically()
	go mainConfigManager.LogConfig()
	mainConfigManager.StartWatch()

	select {}
}

// simulateConfigChanges simulates an app component updating the config every 7 seconds.
func simulateConfigChanges(cm *ConfigManager[Config]) {
	for {
		time.Sleep(6 * time.Second)
		cm.mu.Lock()
		newPort := 8000 + rand.Intn(100)
		updateCount := cm.config.UpdateCount + 1
		cm.v.Set("port", newPort)
		cm.v.Set("updateCount", updateCount)
		if err := cm.v.WriteConfig(); err != nil {
			log.Printf("update failed: write error: %v", err)
			cm.mu.Unlock()
			continue
		}
		if err := cm.v.Unmarshal(&cm.config); err != nil {
			log.Printf("update failed: unmarshal error: %v", err)
		} else {
			log.Printf("updated: port=%d count=%d", newPort, updateCount)
		}
		cm.mu.Unlock()
	}
}

// addRandomKeyPeriodically adds a random extra key to the config every 15 seconds.
func (cm *ConfigManager[T]) addRandomKeyPeriodically() {
	for {
		time.Sleep(15 * time.Second)
		cm.mu.Lock()
		randomKey := fmt.Sprintf("extra_key_%d", rand.Intn(10000))
		randomValue := rand.Intn(100000)
		cm.v.Set(randomKey, randomValue)
		if err := cm.v.WriteConfig(); err != nil {
			log.Printf("config file '%s' random key write error: %v", cm.configPath, err)
		}
		cm.mu.Unlock()
	}
}

// corruptConfigFilePeriodically messes up the config file every 25 seconds.
func (cm *ConfigManager[T]) corruptConfigFilePeriodically() {
	for {
		time.Sleep(25 * time.Second)
		log.Printf("corrupting file: %s", cm.configPath)
		badData := []byte("i am not a valid yaml file")
		if err := os.WriteFile(cm.configPath, badData, 0644); err != nil {
			log.Printf("corrupt write error: %v", err)
		}
	}
}

func postRestoreFunc() {
	log.Println("post-restore function called")
}
