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
		}
		APIError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}.write(w)
		return
	}
	log.WithField("app_eui", app.AppEUI).Info("application created")
	w.WriteHeader(http.StatusCreated)
}

// ApplicationObjectHandler is a http.Handler which handles GET, PUT and
// DELETE request on a single object.
type ApplicationObjectHandler struct {
	Client *loracontrol.Client
}

func (h *ApplicationObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var appEUI lorawan.EUI64
	b, err := hex.DecodeString(id)
	if err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}
	if len(b) != len(appEUI) {
		APIError{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("an AppEUI is exactly %d bytes", len(appEUI)),
		}.write(w)
		return
	}
	copy(appEUI[:], b)

	switch r.Method {
	case "GET":
		h.serveGET(w, r, appEUI)
	case "PUT":
		h.servePUT(w, r, appEUI)
	case "DELETE":
		h.serveDELETE(w, r, appEUI)
	default:
		APIError{
			Code:    http.StatusMethodNotAllowed,
			Message: "method not allowed",
		}.write(w)
	}
}

func (h *ApplicationObjectHandler) serveGET(w http.ResponseWriter, r *http.Request, appEUI lorawan.EUI64) {
	app, err := h.Client.Application().Get(appEUI)
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
	if err := enc.Encode(app); err != nil {
		APIError{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		}.write(w)
	}
}

func (h *ApplicationObjectHandler) servePUT(w http.ResponseWriter, r *http.Request, appEUI lorawan.EUI64) {
	app := new(loracontrol.Application)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(app); err != nil {
		APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		}.write(w)
		return
	}

	if app.AppEUI != appEUI {
		APIError{
			Code:    http.StatusBadRequest,
			Message: "AppEUI in url should match AppEUI in request body",
		}.write(w)
		return
	}

	if err := h.Client.Application().Update(app); err != nil {
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

func (h *ApplicationObjectHandler) serveDELETE(w http.ResponseWriter, r *http.Request, appEUI lorawan.EUI64) {
	err := h.Client.Application().Delete(appEUI)
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
	log.WithField("app_eui", appEUI).Info("application deleted")
	w.WriteHeader(http.StatusNoContent)
}
