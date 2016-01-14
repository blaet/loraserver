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

				Convey("Then the status code is 201 and the application is created", func() {
					So(resp.StatusCode, ShouldEqual, http.StatusCreated)

					out, err := c.Application().Get([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
					So(err, ShouldBeNil)
					So(out, ShouldResemble, app)

					Convey("Then creating the same object again fails", func() {
						resp, err := http.Post(s.URL, "application/json", bytes.NewReader(jsonBytes))
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
					})
				})
			})
		})
	})
}

func TestApplicationObjectHandler(t *testing.T) {
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
			r.Handle("/{id}", &ApplicationObjectHandler{c})
			s := httptest.NewServer(r)

			Convey("Getting a non-existing application returns 404", func() {
				resp, err := http.Get(s.URL + "/0102030405060708")
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
			})

			Convey("Given an application in the database", func() {
				app := &loracontrol.Application{
					AppEUI:      [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
					CallbackURL: "http://foo.bar/",
				}
				So(c.Application().Create(app), ShouldBeNil)

				Convey("Then GET returns the application", func() {
					resp, err := http.Get(s.URL + "/0102030405060708")
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusOK)

					out := new(loracontrol.Application)
					dec := json.NewDecoder(resp.Body)
					So(dec.Decode(out), ShouldBeNil)
					So(out, ShouldResemble, app)
				})

				Convey("When DELETE-ing this application", func() {
					req, err := http.NewRequest("DELETE", s.URL+"/0102030405060708", nil)
					So(err, ShouldBeNil)
					resp, err := http.DefaultClient.Do(req)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusNoContent)

					Convey("Then the application has been removed", func() {
						resp, err := http.Get(s.URL + "/0102030405060708")
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusNotFound)
					})
				})

				Convey("When updating the application", func() {
					app.CallbackURL = "http://example.com/"
					b, err := json.Marshal(app)
					So(err, ShouldBeNil)
					req, err := http.NewRequest("PUT", s.URL+"/0102030405060708", bytes.NewReader(b))
					So(err, ShouldBeNil)
					resp, err := http.DefaultClient.Do(req)
					So(err, ShouldBeNil)
					So(resp.StatusCode, ShouldEqual, http.StatusNoContent)

					Convey("Then the application is updated", func() {
						resp, err := http.Get(s.URL + "/0102030405060708")
						So(err, ShouldBeNil)
						So(resp.StatusCode, ShouldEqual, http.StatusOK)

						out := new(loracontrol.Application)
						dec := json.NewDecoder(resp.Body)
						So(dec.Decode(out), ShouldBeNil)
						So(out, ShouldResemble, app)
					})
				})
			})
		})
	})
}
