package loraserver

import (
	"encoding/json"
	"net/http"
)

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
