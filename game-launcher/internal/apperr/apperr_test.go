package apperr

import (
	"errors"
	"io"
	"testing"
)

func TestErrorFormat(t *testing.T) {
	err := New("parse.status", P{"site": "F95zone", "status": 403}, "%s returned status %d", "F95zone", 403)
	want := `[parse.status {"site":"F95zone","status":403}] F95zone returned status 403`
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	if Code(err) != "parse.status" {
		t.Errorf("Code = %q", Code(err))
	}

	// Без параметров — только код; %w сохраняет исходную ошибку
	err = New("parse.read", nil, "HTML read error: %w", io.ErrUnexpectedEOF)
	if err.Error() != "[parse.read] HTML read error: unexpected EOF" {
		t.Errorf("Error() = %q", err.Error())
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Error("errors.Is не видит обёрнутую ошибку")
	}
	if Code(errors.New("plain")) != "" {
		t.Error("у обычной ошибки не должно быть кода")
	}
}
