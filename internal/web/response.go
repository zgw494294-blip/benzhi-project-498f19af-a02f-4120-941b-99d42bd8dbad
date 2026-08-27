package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("写入 JSON 响应失败", "error", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	body := apiError{Code: "internal_error", Message: "服务暂时无法完成请求"}
	var rule *domain.RuleError
	switch {
	case errors.As(err, &rule):
		status = http.StatusUnprocessableEntity
		body = apiError{Code: rule.Code, Message: rule.Message, Field: rule.Field}
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		body = apiError{Code: "not_found", Message: "未找到指定记录"}
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		body = apiError{Code: "version_conflict", Message: "记录已被他人修改，请刷新后重试"}
	case errors.Is(err, domain.ErrInvalidState):
		status = http.StatusConflict
		body = apiError{Code: "invalid_state", Message: err.Error()}
	case errors.Is(err, domain.ErrAlreadyFrozen):
		status = http.StatusConflict
		body = apiError{Code: "already_frozen", Message: "冻结引用不可再修改"}
	case errors.Is(err, domain.ErrDataConsistency):
		status = http.StatusConflict
		body = apiError{Code: "data_consistency_error", Message: err.Error()}
	case errors.Is(err, domain.ErrCredentialRevoked):
		status = http.StatusConflict
		body = apiError{Code: "credential_already_revoked", Message: "凭据已经撤销，撤销不可逆"}
	}
	if status == http.StatusInternalServerError {
		slog.Error("请求处理失败", "error", err)
	}
	writeJSON(w, status, errorBody{Error: body})
}
