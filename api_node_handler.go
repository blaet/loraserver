package loraserver

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	"github.com/gorilla/mux"
)

// NodeCreateHandler is a http.Handler which creates nodes.
type NodeCreateHandler struct {
	Client *loracontrol.Client
}

func (h *NodeCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	node := loracontrol.Node{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&node); err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}
	if err := h.Client.Node().Create(node); err != nil {
		if err == loracontrol.ErrObjectExists {
			APIError{
				Code:    http.StatusBadRequest,
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
	log.WithField("dev_eui", node.DevEUI).Info("node created")
	w.WriteHeader(http.StatusCreated)
}

// NodeObjectHandler is a http.Handler which handles GET, PUT and
// DELETE request on a single object.
type NodeObjectHandler struct {
	Client *loracontrol.Client
}

func (h *NodeObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var devEUI lorawan.EUI64
	b, err := hex.DecodeString(id)
	if err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}
	if len(b) != len(devEUI) {
		APIError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("a DevEUI is exactly %d bytes", len(devEUI)),
		}.write(w)
		return
	}
	copy(devEUI[:], b)

	switch r.Method {
	case "GET":
		h.serveGET(w, r, devEUI)
	case "PUT":
		h.servePUT(w, r, devEUI)
	case "DELETE":
		h.serveDELETE(w, r, devEUI)
	default:
		APIError{
			Code:    http.StatusMethodNotAllowed,
			Message: "method not allowed",
		}.write(w)
	}
}

func (h *NodeObjectHandler) serveGET(w http.ResponseWriter, r *http.Request, devEUI lorawan.EUI64) {
	node, err := h.Client.Node().Get(devEUI)
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
	if err := enc.Encode(node); err != nil {
		APIError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}.write(w)
	}
}

func (h *NodeObjectHandler) servePUT(w http.ResponseWriter, r *http.Request, devEUI lorawan.EUI64) {
	node := loracontrol.Node{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&node); err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}

	if node.DevEUI != devEUI {
		APIError{
			Code:    http.StatusBadRequest,
			Message: "DevEUI in url should match DevEUI in request body",
		}.write(w)
		return
	}

	if err := h.Client.Node().Update(node); err != nil {
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

	w.WriteHeader(http.StatusNoContent)
}

func (h *NodeObjectHandler) serveDELETE(w http.ResponseWriter, r *http.Request, devEUI lorawan.EUI64) {
	if err := h.Client.Node().Delete(devEUI); err != nil {
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
	log.WithField("dev_eui", devEUI).Info("node deleted")
	w.WriteHeader(http.StatusNoContent)
}
