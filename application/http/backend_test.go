package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	. "github.com/smartystreets/goconvey/convey"
)

type config struct {
	RedisServer   string
	RedisPassword string
}

func getConfig() *config {
	c := &config{
		RedisServer:   "localhost:6379",
		RedisPassword: "",
	}

	if v := os.Getenv("TEST_REDIS_SERVER"); v != "" {
		c.RedisServer = v
	}
	if v := os.Getenv("TEST_REDIS_PASSWORD"); v != "" {
		c.RedisPassword = v
	}

	return c
}

type testApplicationHandler struct {
	responseCode int
	now          time.Time
	data         string
}

func (h *testApplicationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload := struct {
		Time         time.Time `json:"time"`
		GatewayCount int       `json:"gatewayCount"`
		PHYPayload   string    `json:"phyPayload"`
	}{}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if payload.PHYPayload != h.data {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !payload.Time.Equal(h.now) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if payload.GatewayCount != 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(h.responseCode)
}

func TestBackend(t *testing.T) {
	conf := getConfig()
	appSKey := [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}

	Convey("Given a Client, clean Redis database an HTTP application backend", t, func() {
		c, err := loracontrol.NewClient(
			loracontrol.SetStorageBackend(loracontrol.NewRedisBackend(conf.RedisServer, conf.RedisPassword)),
			loracontrol.SetGatewayBackend(&loracontrol.DummyGatewayBackend{}),
			loracontrol.SetApplicationBackend(&Backend{}),
		)
		So(err, ShouldBeNil)
		So(c.Storage().FlushAll(), ShouldBeNil)

		Convey("Given a test application handler server", func() {
			h := &testApplicationHandler{}
			s := httptest.NewServer(h)

			Convey("Given a test RXPackets", func() {
				macPl := lorawan.NewMACPayload(true)
				macPl.FHDR = lorawan.FHDR{
					DevAddr: lorawan.DevAddr([4]byte{1, 1, 1, 1}),
				}
				macPl.FPort = 1
				macPl.FRMPayload = []lorawan.Payload{
					&lorawan.DataPayload{Bytes: []byte("hello")},
				}
				So(macPl.EncryptFRMPayload(appSKey), ShouldBeNil)

				phy := lorawan.NewPHYPayload(true)
				phy.MHDR = lorawan.MHDR{
					MType: lorawan.UnconfirmedDataUp,
					Major: lorawan.LoRaWANR1,
				}
				phy.MACPayload = macPl

				now := time.Now().UTC()
				packets := loracontrol.RXPackets{
					loracontrol.RXPacket{RXInfo: loracontrol.RXInfo{Time: now}, PHYPayload: phy},
					loracontrol.RXPacket{RXInfo: loracontrol.RXInfo{Time: now}, PHYPayload: phy},
				}

				b, err := phy.MarshalBinary()
				So(err, ShouldBeNil)

				h.now = now
				h.data = base64.StdEncoding.EncodeToString(b)

				Convey("Given an application in the database", func() {
					app := &loracontrol.Application{
						AppEUI: lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
						Config: loracontrol.PropertyBag{
							String: map[string]string{
								"callback_url": s.URL,
							},
						},
					}
					So(c.Application().Create(app), ShouldBeNil)

					Convey("When then handler returns 200 or 201, Send does not error", func() {
						for _, code := range []int{200, 201} {
							h.responseCode = code
							So(c.Application().Send(app.AppEUI, packets), ShouldBeNil)
						}
					})

					Convey("When the handler returns != 200 or 201, send returns an error", func() {
						for _, code := range []int{400, 500} {
							h.responseCode = code
							So(c.Application().Send(app.AppEUI, packets), ShouldResemble, fmt.Errorf("application/http: expected 200 or 201 response code, got: %d", code))
						}
					})
				})

				Convey("When sending to a non-existing application, Send returns an error", func() {
					So(c.Application().Send(lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8}, packets), ShouldResemble, loracontrol.ErrObjectDoesNotExist)
				})
			})
		})
	})
}
