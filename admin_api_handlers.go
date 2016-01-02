package loraserver

import "encoding/json"
import "github.com/brocaar/loracontrol"
import "net/http"

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
	c *loracontrol.Client
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
	if err := h.c.Application().Create(app); err != nil {
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
	w.WriteHeader(http.StatusCreated)
}

// NodeCreateHandler is a http.Handler which creates nodes.
type NodeCreateHandler struct {
	c *loracontrol.Client
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
	if err := h.c.Node().Create(node); err != nil {
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
	w.WriteHeader(http.StatusCreated)
}
