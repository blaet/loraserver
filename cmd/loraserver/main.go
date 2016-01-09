package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/loraserver"
	"github.com/codegangsta/cli"
	"github.com/gorilla/mux"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func run(c *cli.Context) {
	// get control client with redis backend
	log.WithField("server", c.String("redis-server")).Info("connecting to redis")
	client, err := loracontrol.NewClient(
		loracontrol.SetRedisBackend(c.String("redis-server"), c.String("redis-password")),
		loracontrol.SetHTTPApplicationBackend(),
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

	udpSendChan := make(chan loraserver.UDPPacket)
	go loraserver.SendGatewayPackets(conn, udpSendChan)
	go loraserver.ReadGatewayPackets(conn, udpSendChan, client)

	// setup admin handler
	r := mux.NewRouter().StrictSlash(true)
	r.Handle("/api/application", &loraserver.ApplicationCreateHandler{Client: client}).Methods("POST")
	r.Handle("/api/application/{id}", &loraserver.ApplicationObjectHandler{Client: client}).Methods("GET", "PUT", "DELETE")
	r.Handle("/api/node", &loraserver.NodeCreateHandler{Client: client}).Methods("POST")
	log.WithField("address", fmt.Sprintf("0.0.0.0:%d", c.Int("admin-port"))).Info("starting admin http api server")
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", c.Int("admin-port")), r))
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
		cli.IntFlag{
			Name:   "admin-port",
			Value:  8000,
			Usage:  "port to bind to for the admin api (HTTP)",
			EnvVar: "ADMIN_PORT",
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
