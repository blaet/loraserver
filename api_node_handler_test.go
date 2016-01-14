package loraserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brocaar/loracontrol"
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