package main

import (
	"fmt"
	"net"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/loraserver"
	"github.com/codegangsta/cli"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func run(c *cli.Context) {
	// get control client with redis backend
	log.WithField("server", c.String("redis-server")).Info("connecting to redis")
	_, err := loracontrol.NewClient(
		loracontrol.SetRedisBackend(c.String("redis-server"), c.String("redis-password")),
	)
	if err != nil {
		log.Fatal(err)
	}

	// setup UDP socket for gateway data
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", c.Int("gw-port")))
	if err != nil {
		log.Fatal(err)
	}
	log.WithField("address", addr).Info("starting gateway udp listener")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	loraserver.ReadGatewayPackets(conn)
}

func main() {
	app := cli.NewApp()
	app.Name = "loraserver"
	app.Usage = "LoRaWAN server which handles uplink and downlink messages to and from the gateway"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:   "gw-port",
			Value:  1680,
			Usage:  "port to bind to for incoming (UDP) gateway packets",
			EnvVar: "GW_PORT",
		},
		cli.StringFlag{
			Name:   "redis-server",
			Value:  "localhost:6379",
			Usage:  "hostname:port of the Redis server",
			EnvVar: "REDIS_SERVER",
		},
		cli.StringFlag{
			Name:   "redis-password",
			Value:  "",
			Usage:  "password of the Redis server",
			EnvVar: "REDIS_PASSWORD",
		},
	}
	app.Action = run
	app.Run(os.Args)
}
