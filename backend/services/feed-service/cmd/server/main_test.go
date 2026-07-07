package server

import "testing"

func TestStartCmdConfigFlagDefault(t *testing.T) {
	flag := StartCmd.PersistentFlags().Lookup("config")
	if flag == nil {
		t.Fatal("expected config flag")
	}
	if flag.DefValue != defaultConfigFile {
		t.Fatalf("config flag default = %q, want %q", flag.DefValue, defaultConfigFile)
	}
}
