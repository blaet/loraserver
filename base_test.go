package loraserver

import (
	"os"

	log "github.com/Sirupsen/logrus"
)

type config struct {
	RedisServer   string
	RedisPassword string
}

func init() {
	log.SetLevel(log.PanicLevel)
}

func getConfig() *config {
	c := &config{
		RedisServer:   "localhost:6379",
		RedisPassword: "",
	}

	if v := os.Getenv("TEST_REDIS_SERVER"); v != "" {
		c.RedisServer = v
	}
	if v := os.Getenv("TEST_REDIS_PASSWORD"); v != "" {
		c.RedisPassword = v
	}

	return c
}
