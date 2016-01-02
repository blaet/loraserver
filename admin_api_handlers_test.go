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

func TestApplicationCreateHandler(t *testing.T) {
	conf := getConfig()

	Convey("Given a Client connected to a clean Redis database", t, func() {
		c, err := loracontrol.NewClient(
			loracontrol.SetRedisBackend(conf.RedisServer, conf.RedisPassword),
			loracontrol.SetHTTPApplicationBackend(),
		)
		So(err, ShouldBeNil)
		So(c.Storage().FlushAll(), ShouldBeNil)

		Convey("Given a test http server serving the handler and test JSON", func() {
			s := httptest.NewServer(&ApplicationCreateHandler{c})
			app := &loracontrol.Application{
				AppEUI:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
				CallbackURL: "http://foo.bar/",
			}
			jsonBytes, err := json.Marshal(app)
			So(err, ShouldBeNil)

			Convey("When posting valid JSON", func() {
				resp, err := http.Post(s.URL, "application/json", bytes.NewReader(jsonBytes))
				So(err, ShouldBeNil)

				Convey("Then the status code should be 201 and the application should be created", func() {
					So(resp.StatusCode, ShouldEqual, http.StatusCreated)

					out, err := c.Application().Get([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
					So(err, ShouldBeNil)
					So(out, ShouldResemble, app)
				})
			})
		})
	})
}
