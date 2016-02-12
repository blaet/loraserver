package loraserver

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
	"github.com/brocaar/lorawan"
	"github.com/gorilla/mux"
)

// NodeSessionCreateHandler is a http.Handler which creates NodeSession objects.
type NodeSessionCreateHandler struct {
	Client *loracontrol.Client
}

func (h *NodeSessionCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	nodeSession := loracontrol.NodeSession{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&nodeSession); err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}
	if err := h.Client.NodeSession().CreateExpire(nodeSession); err != nil {
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
	log.WithField("dev_addr", nodeSession.DevAddr).Info("node-session created")
	w.WriteHeader(http.StatusCreated)
}

// NodeSessionObjectHandler is a http.Handler which handles GET, PUT and
// DELETE requests on a single object.
type NodeSessionObjectHandler struct {
	Client *loracontrol.Client
}

func (h *NodeSessionObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var devAddr lorawan.DevAddr
	b, err := hex.DecodeString(id)
	if err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}

	if len(b) != len(devAddr) {
		APIError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("a DevAddr is exactly %d bytes", len(devAddr)),
		}.write(w)
		return
	}
	copy(devAddr[:], b)

	var status int

	switch r.Method {
	case "GET":
		status, err = h.serveGET(w, r, devAddr)
	case "PUT":
		status, err = h.servePUT(w, r, devAddr)
	case "DELETE":
		status, err = h.serveDELETE(w, r, devAddr)
	default:
		status = http.StatusMethodNotAllowed
	}

	if err != nil {
		APIError{
			Code:    status,
			Message: err.Error(),
		}.write(w)
		return
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
}

func (h *NodeSessionObjectHandler) serveGET(w http.ResponseWriter, r *http.Request, devAddr lorawan.DevAddr) (int, error) {
	nodeSession, err := h.Client.NodeSession().Get(devAddr)
	if err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(nodeSession); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}
func (h *NodeSessionObjectHandler) servePUT(w http.ResponseWriter, r *http.Request, devAddr lorawan.DevAddr) (int, error) {
	nodeSession := loracontrol.NodeSession{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&nodeSession); err != nil {
		return http.StatusBadRequest, err
	}

	if nodeSession.DevAddr != devAddr {
		return http.StatusBadRequest, errors.New("DevAddr should match DevAddr in request body")
	}

	if err := h.Client.NodeSession().UpdateExpire(nodeSession); err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}

	log.WithField("dev_addr", devAddr).Info("node-session updated")
	return http.StatusNoContent, nil
}
func (h *NodeSessionObjectHandler) serveDELETE(w http.ResponseWriter, r *http.Request, devAddr lorawan.DevAddr) (int, error) {
	if err := h.Client.NodeSession().Delete(devAddr); err != nil {
		if err == loracontrol.ErrObjectDoesNotExist {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}

	log.WithField("dev_addr", devAddr).Info("node-session deleted")
	return http.StatusNoContent, nil
}
