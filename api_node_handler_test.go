package loraserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brocaar/loracontrol"
	"github.com/gorilla/mux"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNodeCreateHandler(t *testing.T) {
	conf := getConfig()

	Convey("Given a client connected to a clean Redis database", t, func() {
		c, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(conf.RedisServer, conf.RedisPassword)),
			loracontrol.SetApplicationBackend(&loracontrol.DummyApplicationBackend{}),
			loracontrol.SetGatewayBackend(&loracontrol.DummyGatewayBackend{}),
		)
		So(err, ShouldBeNil)
		So(c.Storage().FlushAll(), ShouldBeNil)

		Convey("Given a test http server serving the handler and test JSON", func() {
			s := httptest.NewServer(&NodeCreateHandler{c})
			node := &loracontrol.Node{
				DevAddr: [4]byte{1, 2, 3, 4},
				AppEUI:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			}
			jsonBytes, err := json.Marshal(node)
			So(err, ShouldBeNil)

			Convey("When posting invalid JSON", func() {
				resp, err := http.Post(s.URL, "application/json", bytes.NewReader(jsonBytes[1:]))
				So(err, ShouldBeNil)

				Convey("Then a 401 status is returned", func() {
					So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
				})
			})

			Convey("When posting valid JSON", func() {
				resp, err := http.Post(s.URL, "application/json", bytes.NewReader(jsonBytes))
				So(err, ShouldBeNil)

				Convey("Then the status code is 201 and the node is created", func() {
					So(resp.StatusCode, ShouldEqual, http.StatusCreated)

					out, err := c.Node().Get([4]byte{1, 2, 3, 4})
					So(err, ShouldBeNil)
					So(out, ShouldResemble, node)

					Convey("Then creating the same node again fails", func() {
						resp, err := http.Post(s.URL, "application/json", bytes.NewReader(jsonBytes))
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
					})
				})
			})
		})
	})
}

func TestNodeObjectHandler(t *testing.T) {
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
			r.Handle("/{id}", &NodeObjectHandler{c})
			s := httptest.NewServer(r)

			Convey("Getting a non-existing node returns 404", func() {
				resp, err := http.Get(s.URL + "/01020304")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
			})

			Convey("Getting an invalid node id returns 401", func() {
				resp, err := http.Get(s.URL + "/0102030")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
			})

			Convey("Getting a node with an id which is too short returns 401", func() {
				resp, err := http.Get(s.URL + "/010203")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
			})

			Convey("Given a node in the database", func() {
				node := &loracontrol.Node{
					DevAddr: [4]byte{1, 2, 3, 4},
					AppEUI:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
				}
				So(c.Node().Create(node), ShouldBeNil)

				Convey("Then GET returns the node", func() {
					resp, err := http.Get(s.URL + "/01020304")
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)

					out := new(loracontrol.Node)
					dec := json.NewDecoder(resp.Body)
					So(dec.Decode(out), ShouldBeNil)
					So(out, ShouldResemble, node)
				})

				Convey("When DELETE-ing this node", func() {
					req, err := http.NewRequest("DELETE", s.URL+"/01020304", nil)
					So(err, ShouldBeNil)
					resp, err := http.DefaultClient.Do(req)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusNoContent)

					Convey("Then the node has been deleted", func() {
						resp, err := http.Get(s.URL + "/01020304")
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
					})
				})

				Convey("When updating the node", func() {
					node.AppEUI = [8]byte{1, 1, 1, 1, 1, 1, 1, 1}
					b, err := json.Marshal(node)
					So(err, ShouldBeNil)
					req, err := http.NewRequest("PUT", s.URL+"/01020304", bytes.NewReader(b))
					So(err, ShouldBeNil)
					resp, err := http.DefaultClient.Do(req)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusNoContent)

					Convey("Then the node is updated", func() {
						resp, err := http.Get(s.URL + "/01020304")
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusOK)

						out := new(loracontrol.Node)
						dec := json.NewDecoder(resp.Body)
						So(dec.Decode(out), ShouldBeNil)
						So(out, ShouldResemble, node)
					})

					Convey("When updating with invalid JSON then a 401 is returned", func() {
						req, err := http.NewRequest("PUT", s.URL+"/01020304", bytes.NewReader(b[1:]))
						So(err, ShouldBeNil)
						resp, err := http.DefaultClient.Do(req)
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
					})

					Convey("When the id in the url does not match the id in the body then a 401 is returned", func() {
						node.DevAddr = [4]byte{1, 2, 3, 3}
						b, err := json.Marshal(node)
						So(err, ShouldBeNil)
						req, err := http.NewRequest("PUT", s.URL+"/01020304", bytes.NewReader(b))
						So(err, ShouldBeNil)
						resp, err := http.DefaultClient.Do(req)
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
					})

				})

				Convey("Requesting with an invalid method returns 405", func() {
					req, err := http.NewRequest("PATCH", s.URL+"/01020304", nil)
					So(err, ShouldBeNil)
					resp, err := http.DefaultClient.Do(req)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusMethodNotAllowed)
				})
			})
		})
	})
}
