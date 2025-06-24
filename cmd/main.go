package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	gcm "github.com/dimalukas/go-config-manager"
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

	mainConfigManager, err := gcm.NewConfigManager(
		"config.yaml", "config.backup.yaml",
		mainConfigKeys,
		config,
		5*time.Second,
		3*time.Second,
		preRestoreFunc,
		postRestoreFunc,
	)
	if err != nil {
		log.Fatalf("init failed: %v", err)
	}

	go simulateConfigChanges(mainConfigManager)
	go addRandomKeyPeriodically(mainConfigManager)
	go corruptConfigFilePeriodically(mainConfigManager)
	go mainConfigManager.LogConfig()
	mainConfigManager.StartWatch()

	select {}
}

// simulateConfigChanges simulates an app component updating the config every 7 seconds.
func simulateConfigChanges(cm *gcm.ConfigManager[Config]) {
	for {
		time.Sleep(6 * time.Second)
		newPort := 8000 + rand.Intn(100)
		updateCount := cm.Config.UpdateCount + 1
		cm.SetKey("port", newPort)
		cm.SetKey("updateCount", updateCount)
		if err := cm.SaveConfig(true); err != nil {
			log.Printf("update failed: write error: %v", err)
			continue
		} else {
			log.Printf("updated: port=%d count=%d", newPort, updateCount)
		}
	}
}

// addRandomKeyPeriodically adds a random extra key to the config every 15 seconds.
func addRandomKeyPeriodically(cm *gcm.ConfigManager[Config]) {
	for {
		time.Sleep(15 * time.Second)
		randomKey := fmt.Sprintf("extra_key_%d", rand.Intn(10000))
		randomValue := rand.Intn(100000)
		cm.SetKey(randomKey, randomValue)
		if err := cm.SaveConfig(true); err != nil {
			log.Printf("config file '%s' random key write error: %v", cm.ConfigPath, err)
		}
	}
}

// corruptConfigFilePeriodically messes up the config file every 25 seconds.
func corruptConfigFilePeriodically(cm *gcm.ConfigManager[Config]) {
	for {
		time.Sleep(25 * time.Second)
		log.Printf("corrupting file: %s", cm.ConfigPath)
		badData := []byte("i am not a valid yaml file")
		if err := os.WriteFile(cm.ConfigPath, badData, 0644); err != nil {
			log.Printf("corrupt write error: %v", err)
		}
	}
}

func postRestoreFunc() {
	log.Println("post-restore function called")
}

func preRestoreFunc() {
	log.Println("pre-restore function called")
}