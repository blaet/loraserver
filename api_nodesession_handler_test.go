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

func TestNodeSessionCreateHandler(t *testing.T) {
	conf := getConfig()

	Convey("Given a Client and Redis storage backend", t, func() {
		c, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(conf.RedisServer, conf.RedisPassword)),
			loracontrol.SetApplicationBackend(&loracontrol.DummyApplicationBackend{}),
			loracontrol.SetGatewayBackend(&loracontrol.DummyGatewayBackend{}),
		)
		So(err, ShouldBeNil)
		So(c.Storage().FlushAll(), ShouldBeNil)

		Convey("Given a test http server serving the handler and test json", func() {
			s := httptest.NewServer(&NodeSessionCreateHandler{c})
			nodeSession := loracontrol.NodeSession{
				DevAddr: [4]byte{1, 2, 3, 4},
				DevEUI:  [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
			}
			jsonBytes, err := json.Marshal(nodeSession)
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

				Convey("Then the status code is 201 and the node-session is created", func() {
					So(resp.StatusCode, ShouldEqual, http.StatusCreated)

					out, err := c.NodeSession().Get(nodeSession.DevAddr)
					So(err, ShouldBeNil)
					So(out, ShouldResemble, nodeSession)

					Convey("Then creating the same node-session again fails", func() {
						resp, err := http.Post(s.URL, "application/json", bytes.NewReader(jsonBytes))
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
					})
				})
			})
		})
	})
}

func TestNodeSessionObjectHandler(t *testing.T) {
	conf := getConfig()

	Convey("Given a Client and Redis storage backend", t, func() {
		c, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(conf.RedisServer, conf.RedisPassword)),
			loracontrol.SetApplicationBackend(&loracontrol.DummyApplicationBackend{}),
			loracontrol.SetGatewayBackend(&loracontrol.DummyGatewayBackend{}),
		)
		So(err, ShouldBeNil)
		So(c.Storage().FlushAll(), ShouldBeNil)

		Convey("Given a test server serving the handler", func() {
			r := mux.NewRouter()
			r.Handle("/{id}", &NodeSessionObjectHandler{c})
			s := httptest.NewServer(r)

			Convey("Getting a non-existing node-session returns a 404", func() {
				resp, err := http.Get(s.URL + "/01020304")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
			})

			Convey("Getting an invalid node-session id returns a 401", func() {
				resp, err := http.Get(s.URL + "/0102030")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
			})

			Convey("Getting a node-session with an id which is too short returns a 401", func() {
				resp, err := http.Get(s.URL + "/010203")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
			})

			Convey("Given a node-session in the database", func() {
				nodeSession := loracontrol.NodeSession{
					DevAddr: [4]byte{1, 2, 3, 4},
					DevEUI:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
				}
				So(c.NodeSession().CreateExpire(nodeSession), ShouldBeNil)

				Convey("Then GET returns the node-session", func() {
					resp, err := http.Get(s.URL + "/01020304")
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)

					out := loracontrol.NodeSession{}
					dec := json.NewDecoder(resp.Body)
					So(dec.Decode(&out), ShouldBeNil)
					So(out, ShouldResemble, nodeSession)
				})

				Convey("When DELETE-ing the node-session", func() {
					req, err := http.NewRequest("DELETE", s.URL+"/01020304", nil)
					So(err, ShouldBeNil)
					resp, err := http.DefaultClient.Do(req)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusNoContent)

					Convey("Then the node-session has been deleted", func() {
						resp, err := http.Get(s.URL + "/01020304")
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
					})
				})

				Convey("When UPDATE-ing the node-session", func() {
					nodeSession.DevEUI = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
					b, err := json.Marshal(nodeSession)
					So(err, ShouldBeNil)
					req, err := http.NewRequest("PUT", s.URL+"/01020304", bytes.NewReader(b))
					So(err, ShouldBeNil)
					resp, err := http.DefaultClient.Do(req)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusNoContent)

					Convey("Then the node-session is updated", func() {
						resp, err := http.Get(s.URL + "/01020304")
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusOK)

						out := loracontrol.NodeSession{}
						dec := json.NewDecoder(resp.Body)
						So(dec.Decode(&out), ShouldBeNil)
						So(out, ShouldResemble, nodeSession)
					})

					Convey("When UPDATE-ing with invalid JSON then a 401 is returned", func() {
						req, err := http.NewRequest("PUT", s.URL+"/01020304", bytes.NewReader(b[1:]))
						So(err, ShouldBeNil)
						resp, err := http.DefaultClient.Do(req)
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
					})

					Convey("When the id in the url does not match the id in they body then a 401 is returned", func() {
						nodeSession.DevAddr = [4]byte{1, 1, 1, 1}
						b, err := json.Marshal(nodeSession)
						So(err, ShouldBeNil)
						req, err := http.NewRequest("PUT", s.URL+"/01020304", bytes.NewReader(b))
						So(err, ShouldBeNil)
						resp, err := http.DefaultClient.Do(req)
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
					})
				})

				Convey("When requesting an invalid method, the a 405 is returned", func() {
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
