package main

import (
	"reflect"
	"testing"
)

// Повторный парсинг не должен стирать теги, добавленные вручную
func TestMergeTags(t *testing.T) {
	got := mergeTags([]string{"Мои любимые", "romance"}, []string{"Romance", "2dcg"})
	if want := []string{"Мои любимые", "romance", "2dcg"}; !reflect.DeepEqual(got, want) {
		t.Errorf("mergeTags = %v, want %v", got, want)
	}
	if got := mergeTags(nil, nil); len(got) != 0 {
		t.Errorf("mergeTags(nil, nil) = %v", got)
	}
}
