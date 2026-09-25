// Package apperr — ошибки с машинным кодом для перевода на фронтенде.
//
// Wails передаёт в JS только текст ошибки, поэтому код и параметры кодируются в начале
// сообщения: `[parse.status {"site":"F95zone","status":403}] F95zone returned status 403`.
// Фронтенд (i18n.js → localizeError) показывает перевод ключа `err.<code>`, а английский
// хвост остаётся для логов и как запасной текст, если перевода нет.
package apperr

import (
	"encoding/json"
	"errors"
	"fmt"
)

// P — параметры для подстановки в перевод ({site}, {status}, …).
type P map[string]any

// Error — ошибка с кодом. Unwrap отдаёт исходную ошибку (errors.Is/As работают).
type Error struct {
	Code   string
	Params P
	Msg    string // английский текст (без префикса)
	Err    error
}

func (e *Error) Error() string {
	prefix := e.Code
	if len(e.Params) > 0 {
		if b, err := json.Marshal(e.Params); err == nil {
			prefix += " " + string(b)
		}
	}
	return "[" + prefix + "] " + e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

// New — ошибка с кодом; msg форматируется как в fmt.Errorf (поддерживает %w).
func New(code string, params P, format string, args ...any) error {
	wrapped := fmt.Errorf(format, args...)
	return &Error{Code: code, Params: params, Msg: wrapped.Error(), Err: errors.Unwrap(wrapped)}
}

// Code — код ошибки (или "", если ошибка без кода).
func Code(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}
