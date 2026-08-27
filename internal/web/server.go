package web

import (
	"embed"
	"io/fs"
	"net/http"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/application"
)

//go:embed webassets/*
var assets embed.FS

type Server struct {
	service *application.Service
	handler http.Handler
}

func New(service *application.Service) *Server {
	s := &Server{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.HandleRoot)
	mux.HandleFunc("GET /workspace", s.HandleWorkspace)
	static, _ := fs.Sub(assets, "webassets")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(static))))
	mux.HandleFunc("GET /api/v1/cases", s.HandleListCases)
	mux.HandleFunc("POST /api/v1/cases", s.HandleCreateCase)
	mux.HandleFunc("GET /api/v1/cases/{id}", s.HandleGetCase)
	mux.HandleFunc("POST /api/v1/cases/{id}/evidence", s.HandleAddEvidence)
	mux.HandleFunc("GET /api/v1/cases/{id}/evidence/trends", s.HandleEvidenceTrends)
	mux.HandleFunc("POST /api/v1/cases/{id}/assessment", s.HandleAssessRisk)
	mux.HandleFunc("POST /api/v1/cases/{id}/plans", s.HandleSubmitPlan)
	mux.HandleFunc("POST /api/v1/cases/{id}/trials", s.HandleRecordTrial)
	mux.HandleFunc("POST /api/v1/cases/{id}/trials/start", s.HandleStartTrial)
	mux.HandleFunc("POST /api/v1/cases/{id}/trials/observations", s.HandleAppendTrialObservation)
	mux.HandleFunc("POST /api/v1/cases/{id}/review", s.HandleReview)
	mux.HandleFunc("POST /api/v1/cases/{id}/freeze", s.HandleFreeze)
	mux.HandleFunc("POST /api/v1/cases/{id}/credentials", s.HandleIssueCredential)
	mux.HandleFunc("POST /api/v1/cases/{id}/credentials/revoke", s.HandleRevokeCredential)
	mux.HandleFunc("GET /api/v1/cases/{id}/audit", s.HandleAudit)
	mux.HandleFunc("GET /api/v1/credentials/{number}/verify", s.HandleVerifyCredential)
	s.handler = middleware(mux)
	return s
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/workspace", http.StatusTemporaryRedirect)
}

func (s *Server) HandleWorkspace(w http.ResponseWriter, _ *http.Request) {
	data, err := assets.ReadFile("webassets/index.html")
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
