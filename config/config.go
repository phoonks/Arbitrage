package config

import (
	"sync"
)

var (
	lock         sync.RWMutex
	globalConfig Config
)

type Config struct {
	AppServerPort string `split_words:"true" default:"8080"`
}

func Enviroment() Config {
	lock.RLock()
	defer lock.RUnlock()

	return globalConfig
}
