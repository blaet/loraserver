package loraserver

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	"github.com/gorilla/mux"
)

// GatewayObjectHandler is a http.Handler which handles requests
// on a single object.
type GatewayObjectHandler struct {
	Client *loracontrol.Client
}

func (h *GatewayObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		APIError{
			Code:    http.StatusInternalServerError,
			Message: "no id parameter",
		}.write(w)
		return
	}

	var mac lorawan.EUI64
	b, err := hex.DecodeString(id)
	if err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}
	if len(b) != len(mac) {
		APIError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("a gateway MAC is exactly %d bytes", len(mac)),
		}.write(w)
		return
	}
	copy(mac[:], b)

	switch r.Method {
	case "GET":
		h.serveGET(w, r, mac)
	default:
		APIError{
			Code:    http.StatusMethodNotAllowed,
			Message: "method not allowed",
		}.write(w)
	}
}

func (h *GatewayObjectHandler) serveGET(w http.ResponseWriter, r *http.Request, mac lorawan.EUI64) {
	gw, err := h.Client.Gateway().Get(mac)
	if err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			APIError{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			}.write(w)
			return
		}
		APIError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}.write(w)
		return
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(gw); err != nil {
		APIError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}.write(w)
	}
}
