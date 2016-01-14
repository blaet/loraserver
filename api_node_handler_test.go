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
			loracontrol.SetRedisBackend(conf.RedisServer, conf.RedisPassword),
			loracontrol.SetHTTPApplicationBackend(),
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
			loracontrol.SetRedisBackend(conf.RedisServer, conf.RedisPassword),
			loracontrol.SetHTTPApplicationBackend(),
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
				})
			})
		})
	})
}
