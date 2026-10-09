package api

import "net/http"

// Provisório: removido à medida que cada recurso é implementado.
func (s *Server) recurrenceRoutes(mux *http.ServeMux, wallet routeWrap) {}
func (s *Server) settlementRoutes(mux *http.ServeMux, wallet routeWrap) {}
func (s *Server) budgetRoutes(mux *http.ServeMux, wallet routeWrap)     {}
func (s *Server) summaryRoutes(mux *http.ServeMux, wallet routeWrap)    {}
