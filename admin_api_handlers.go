package loraserver

import (
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/brocaar/loracontrol"
)

// APIError represents the API error model.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e APIError) write(w http.ResponseWriter) error {
	w.WriteHeader(e.Code)
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	w.Write(b)
	return nil
}

// ApplicationCreateHandler is a http.Handler which creates applications.
type ApplicationCreateHandler struct {
	Client *loracontrol.Client
}

func (h *ApplicationCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	app := new(loracontrol.Application)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(app); err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}
	if err := h.Client.Application().Create(app); err != nil {
		if err == loracontrol.ErrObjectExists {
			APIError{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			}.write(w)
			return
		} else {
			APIError{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			}.write(w)
			return
		}
	}
	log.WithField("app_eui", app.AppEUI).Info("application created")
	w.WriteHeader(http.StatusCreated)
}

// NodeCreateHandler is a http.Handler which creates nodes.
type NodeCreateHandler struct {
	Client *loracontrol.Client
}

func (h *NodeCreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	node := new(loracontrol.Node)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(node); err != nil {
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
		} else {
			APIError{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			}.write(w)
			return
		}
	}
	log.WithField("dev_addr", node.DevAddr).Info("node created")
	w.WriteHeader(http.StatusCreated)
}
