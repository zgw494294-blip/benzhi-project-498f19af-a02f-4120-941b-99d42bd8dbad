package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

type selfcheckClient struct {
	base   string
	client *http.Client
}

func runSelfcheck(addr string) error {
	runner := &selfcheckClient{
		base:   "http://" + addr,
		client: &http.Client{Timeout: 5 * time.Second},
	}
	var c domain.ConservationCase
	if err := runner.post("/api/v1/cases", map[string]any{
		"idempotencyKey":      "selfcheck-create",
		"caveCode":            "SELF-CAVE-01",
		"muralZone":           "自检壁画区",
		"materialSensitivity": "high",
		"discoveredAt":        time.Now().Add(-24 * time.Hour).UTC(),
		"owner":               "自检保护技术员",
	}, &c); err != nil {
		return err
	}
	if c.Status != domain.StatusEvidenceCollecting || c.Version != 1 {
		return fmt.Errorf("建档状态异常: %s v%d", c.Status, c.Version)
	}
	if err := runner.post("/api/v1/cases/"+c.ID+"/evidence", withMeta(c, "evidence", map[string]any{
		"actor": "自检保护技术员", "zoneCode": "Z-SAFE", "samplePoints": []string{"SP-1", "SP-2"},
		"microscopyFinding": "观察到活性菌丝", "cultureFinding": "培养结果阳性", "imageDigest": "sha256:selfcheck-image",
		"coveragePercent": 18.0, "activityScore": 7.5,
	}), &c); err != nil {
		return err
	}
	if len(c.Evidence) != 1 || c.Evidence[0].RiskLevel == "" {
		return fmt.Errorf("证据风险未形成")
	}
	if err := runner.post("/api/v1/cases/"+c.ID+"/assessment", withMeta(c, "assessment", map[string]any{
		"actor": "自检微生物评估员", "assessor": "自检微生物评估员",
	}), &c); err != nil {
		return err
	}
	if c.Status != domain.StatusRiskAssessed {
		return fmt.Errorf("评估状态异常: %s", c.Status)
	}
	if err := runner.post("/api/v1/cases/"+c.ID+"/plans", withMeta(c, "plan", map[string]any{
		"actor":             "自检微生物评估员",
		"zoneInstructions":  []map[string]any{{"zoneCode": "Z-SAFE", "cleaningMedium": "去离子水", "concentration": 20.0, "contactMinutes": 8}},
		"isolationMeasures": []string{"封闭作业区", "独立工具"}, "stopConditions": []string{"色差超过3", "材料起翘"},
		"rationale": "以低干预参数降低活性并保护颜料层",
	}), &c); err != nil {
		return err
	}
	started := time.Now().Add(-73 * time.Hour).UTC()
	if err := runner.post("/api/v1/cases/"+c.ID+"/trials", withMeta(c, "trial", map[string]any{
		"actor": "自检保护技术员", "plotCode": "PLOT-SAFE", "startedAt": started,
		"baseline": "试验前材料稳定", "baselineActivity": 8.0,
		"observations": []map[string]any{
			{"observedAt": started, "hoursSinceStart": 0.0, "colorDelta": 0.0, "activityScore": 8.0, "note": "基线"},
			{"observedAt": time.Now().UTC(), "hoursSinceStart": 73.0, "colorDelta": 1.1, "activityScore": 2.0, "note": "末次观察"},
		},
		"deviations": []any{},
	}), &c); err != nil {
		return err
	}
	if c.Status != domain.StatusReviewPending {
		return fmt.Errorf("观察窗未推进复核: %s", c.Status)
	}
	if err := runner.post("/api/v1/cases/"+c.ID+"/review", withMeta(c, "review", map[string]any{
		"actor": "自检责任复核员", "reviewer": "自检责任复核员", "decision": "approve",
		"notes": "观察完整，无遗留偏差", "deviations": []any{},
	}), &c); err != nil {
		return err
	}
	if err := runner.post("/api/v1/cases/"+c.ID+"/freeze", withMeta(c, "freeze", map[string]any{
		"actor": "自检责任复核员", "frozenBy": "自检责任复核员",
	}), &c); err != nil {
		return err
	}
	if c.FrozenManifest == nil || c.FrozenManifest.ManifestDigest == "" {
		return fmt.Errorf("冻结摘要为空")
	}
	if err := runner.post("/api/v1/cases/"+c.ID+"/credentials", withMeta(c, "credential", map[string]any{
		"actor": "自检责任复核员", "allowedZones": []string{"Z-SAFE"}, "conditions": []string{"保持环境稳定", "每周巡检"},
		"issuedBy": "自检责任复核员", "validUntil": time.Now().Add(365 * 24 * time.Hour).UTC(),
	}), &c); err != nil {
		return err
	}
	if c.Status != domain.StatusCredentialIssued || c.Credential == nil {
		return fmt.Errorf("凭据未签发")
	}
	var verification struct {
		Valid   bool   `json:"valid"`
		Message string `json:"message"`
	}
	if err := runner.get("/api/v1/credentials/"+c.Credential.CredentialNo+"/verify", &verification); err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("凭据验真未通过: %s", verification.Message)
	}
	var audit struct {
		Items []domain.AuditEvent `json:"items"`
	}
	if err := runner.get("/api/v1/cases/"+c.ID+"/audit", &audit); err != nil {
		return err
	}
	if len(audit.Items) != 8 {
		return fmt.Errorf("审计事件数量异常: %d", len(audit.Items))
	}
	for index, event := range audit.Items {
		if event.Sequence != int64(index+1) {
			return fmt.Errorf("审计序号不连续")
		}
		if index > 0 && event.PreviousHash != audit.Items[index-1].EventHash {
			return fmt.Errorf("审计摘要链不连续")
		}
	}
	return nil
}

func withMeta(c domain.ConservationCase, key string, values map[string]any) map[string]any {
	values["expectedVersion"] = c.Version
	values["idempotencyKey"] = "selfcheck-" + key
	return values
}

func (c *selfcheckClient) post(path string, payload any, target any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	return c.do(request, target)
}

func (c *selfcheckClient) get(path string, target any) error {
	request, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	return c.do(request, target)
}

func (c *selfcheckClient) do(request *http.Request, target any) error {
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d: %s", request.Method, request.URL.Path, response.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("解码 %s: %w", request.URL.Path, err)
	}
	return nil
}
