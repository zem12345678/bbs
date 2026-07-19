package config

import "testing"

func TestSkipNacos(t *testing.T) {
	t.Setenv("BBS_FILE_SKIP_NACOS", "")
	if skipNacos() {
		t.Fatal("skipNacos() = true without override")
	}
	t.Setenv("BBS_FILE_SKIP_NACOS", "true")
	if !skipNacos() {
		t.Fatal("skipNacos() = false with BBS_FILE_SKIP_NACOS=true")
	}
}
