package rest

import (
	"net/http"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
)

// Services holds all the services needed by the REST API
type Services struct {
	CallRouting callrouting.Service
	Bidding     bidding.Service
	Telephony   telephony.Service
	Fraud       fraud.Service
}

// RegisterHandlers registers all REST API handlers
func RegisterHandlers(router interface{}, services Services) {
	// TODO: Implement REST API handlers
	// This is a stub implementation to satisfy compilation
}

// Handler is a generic HTTP handler type
type Handler struct {
	Services Services
}

// NewHandler creates a new REST API handler
func NewHandler(services Services) *Handler {
	return &Handler{
		Services: services,
	}
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement request routing
	w.WriteHeader(http.StatusNotImplemented)
}
