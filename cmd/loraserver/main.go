package main

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/blaet/loraserver"
	apphttp "github.com/blaet/loraserver/application/http"
	"github.com/blaet/loraserver/gateway/semtech"
	"github.com/brocaar/loracontrol"
	"github.com/codegangsta/cli"
	"github.com/gorilla/mux"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func run(c *cli.Context) {
	// start gateway backend
	gw, err := semtech.NewBackend(c.Int("gw-port"))
	if err != nil {
		log.Fatal(err)
	}
	defer gw.Close()

	// get control client with redis backend
	log.WithField("server", c.String("redis-server")).Info("connecting to redis")
	client, err := loracontrol.NewClient(
		loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(c.String("redis-server"), c.String("redis-password"))),
		loracontrol.SetGatewayBackend(gw),
		loracontrol.SetApplicationBackend(&apphttp.Backend{}),
	)
	if err != nil {
		log.Fatal(err)
	}

	go loraserver.HandleGatewayPackets(client)

	// setup admin handler
	r := mux.NewRouter().StrictSlash(true)
	r.Handle("/api/application", &loraserver.ApplicationCreateHandler{Client: client}).Methods("POST")
	r.Handle("/api/application/{id}", &loraserver.ApplicationObjectHandler{Client: client}).Methods("GET", "PUT", "DELETE")
	r.Handle("/api/node", &loraserver.NodeCreateHandler{Client: client}).Methods("POST")
	r.Handle("/api/node/{id}", &loraserver.NodeObjectHandler{Client: client}).Methods("GET", "PUT", "DELETE")
	r.Handle("/api/nodesession", &loraserver.NodeSessionCreateHandler{Client: client}).Methods("POST")
	r.Handle("/api/nodesession/{id}", &loraserver.NodeSessionObjectHandler{Client: client}).Methods("GET", "PUT", "DELETE")
	r.Handle("/api/gateway/{id}", &loraserver.GatewayObjectHandler{Client: client}).Methods("GET")
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
