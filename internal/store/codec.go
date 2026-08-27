package store

import (
	"encoding/json"
	"fmt"

	"benzhi-project-498f19af-a02f-4120-941b-99d42bd8dbad/internal/domain"
)

func marshal(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("编码持久化数据: %w", err)
	}
	return data, nil
}

func unmarshalCase(data []byte) (domain.ConservationCase, error) {
	var c domain.ConservationCase
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("解码处置案: %w", err)
	}
	return c, nil
}

func unmarshalCredential(data []byte) (domain.SafetyCredential, error) {
	var credential domain.SafetyCredential
	if err := json.Unmarshal(data, &credential); err != nil {
		return credential, fmt.Errorf("解码凭据: %w", err)
	}
	return credential, nil
}

func unmarshalManifest(data []byte) (domain.FrozenManifest, error) {
	var manifest domain.FrozenManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("解码冻结清单: %w", err)
	}
	return manifest, nil
}
