package main

import "testing"

func TestConfigDefaultsAndPORT(t *testing.T) {
	t.Setenv("PORT", "")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != "127.0.0.1:19081" {
		t.Fatalf("default address %s", cfg.addr)
	}
	t.Setenv("PORT", "19123")
	cfg, err = parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != "127.0.0.1:19123" {
		t.Fatalf("PORT address %s", cfg.addr)
	}
	cfg, err = parseConfig([]string{"-addr=127.0.0.1:19234"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.addr != "127.0.0.1:19234" {
		t.Fatalf("flag should win: %s", cfg.addr)
	}
}

func TestConfigRejectsPublicListener(t *testing.T) {
	t.Setenv("PORT", "")
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}); err == nil {
		t.Fatal("public listener should be rejected")
	}
}
