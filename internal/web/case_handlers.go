package web

import (
	"net/http"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
)

func (s *Server) HandleListCases(w http.ResponseWriter, r *http.Request) {
	items, err := s.service.ListCases(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) HandleCreateCase(w http.ResponseWriter, r *http.Request) {
	var command application.CreateCaseCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.CreateCase(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/cases/"+result.ID)
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.GetCase(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleAddEvidence(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.AddEvidenceCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.AddEvidence(r.Context(), id, command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleAssessRisk(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var command application.AssessRiskCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.AssessRisk(r.Context(), id, command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) HandleEvidenceTrends(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := s.service.EvidenceTrends(r.Context(), id, r.URL.Query().Get("zoneCode"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": result})
}
