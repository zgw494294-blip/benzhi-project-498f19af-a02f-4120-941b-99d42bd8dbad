package web

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

const maxRequestBody = 1 << 20

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return domain.NewRuleError("content_type_required", "Content-Type 必须为 application/json", "Content-Type")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return domain.NewRuleError("body_too_large", "请求体不得超过 1 MiB", "body")
		}
		return domain.NewRuleError("invalid_json", "JSON 请求体无效："+err.Error(), "body")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.NewRuleError("multiple_json_values", "请求体只能包含一个 JSON 对象", "body")
	}
	return nil
}

func pathID(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		return "", domain.ErrNotFound
	}
	return id, nil
}
