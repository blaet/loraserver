package loraserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	"github.com/gorilla/mux"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGatewayObjectHandler(t *testing.T) {
	conf := getConfig()

	Convey("Given a Client connected to a clean Redis database", t, func() {
		c, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(conf.RedisServer, conf.RedisPassword)),
			loracontrol.SetApplicationBackend(&loracontrol.DummyApplicationBackend{}),
			loracontrol.SetGatewayBackend(&loracontrol.DummyGatewayBackend{}),
		)
		So(err, ShouldBeNil)
		So(c.Storage().FlushAll(), ShouldBeNil)

		Convey("Given a test server serving the handler", func() {
			r := mux.NewRouter()
			r.Handle("/{id}", &GatewayObjectHandler{c})
			s := httptest.NewServer(r)

			Convey("Getting a non-existing gateway returns a 404", func() {
				resp, err := http.Get(s.URL + "/0102030405060708")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
			})

			Convey("Getting an invalid gateway id returns a 401", func() {
				resp, err := http.Get(s.URL + "/010203040506070")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
			})

			Convey("Getting a gateway with an id which is too short returns 401", func() {
				resp, err := http.Get(s.URL + "/01020304050607")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
			})

			Convey("Given a gateway in the database", func() {
				gw := &loracontrol.Gateway{
					UpdatedAt: time.Now().UTC(),
					MAC:       lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
				}
				So(c.Gateway().Upsert(gw), ShouldBeNil)

				Convey("Then GET returns the gateway", func() {
					resp, err := http.Get(s.URL + "/0102030405060708")
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)

					out := new(loracontrol.Gateway)
					dec := json.NewDecoder(resp.Body)
					So(dec.Decode(out), ShouldBeNil)
					So(out, ShouldResemble, gw)
				})
			})
		})
	})
}
