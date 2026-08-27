package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound          = errors.New("未找到记录")
	ErrConflict          = errors.New("版本冲突")
	ErrInvalidState      = errors.New("当前状态不允许此操作")
	ErrRuleBlocked       = errors.New("领域规则阻止操作")
	ErrAlreadyFrozen     = errors.New("证据已经冻结")
	ErrInvalidInput      = errors.New("输入无效")
	ErrCredentialFail    = errors.New("凭据验真失败")
	ErrDataConsistency   = errors.New("数据一致性错误")
	ErrCredentialRevoked = errors.New("凭据已经撤销")
)

type RuleError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (e *RuleError) Error() string { return e.Message }
func (e *RuleError) Unwrap() error { return ErrRuleBlocked }

func NewRuleError(code, message, field string) error {
	return &RuleError{Code: code, Message: message, Field: field}
}

type StateError struct {
	Action string
	State  CaseStatus
}

func (e *StateError) Error() string {
	return fmt.Sprintf("状态 %s 不允许执行 %s", e.State, e.Action)
}

func (e *StateError) Unwrap() error { return ErrInvalidState }
