//go:build !windows

package main

import "errors"

// На других ОС глобальные горячие клавиши пока не поддерживаются.
type hotkeyManager struct{}

func (h *hotkeyManager) Register(mods, vk uint32, onPress func()) error {
	return errors.New("global hotkeys are supported only on Windows")
}

func (h *hotkeyManager) Unregister() {}
