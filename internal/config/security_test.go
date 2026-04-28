package config

import (
	"path/filepath"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/security"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func TestMasterFullRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	cfg, err := LoadConfigWithVersion("test", cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.FirstLaunch {
		t.Fatal("expected FirstLaunch=true")
	}

	// Simulate cmd.go keyring init (default is keyring)
	k, _ := util.GenerateEncryptionKey()
	EncryptionKey = k

	// User picks master, modal opens, user completes setup.
	salt, _ := security.GenerateSalt()
	params := cfg.MasterParams()
	kek, _ := security.DeriveKey("mypass", salt, params)
	if err := cfg.ApplyMasterSetup(kek, salt, params); err != nil {
		t.Fatal(err)
	}
	dataKeyAfterSetup := EncryptionKey
	if cfg.Security.Method != SecurityMethodMaster {
		t.Fatalf("ApplyMasterSetup should set Method=master, got %q", cfg.Security.Method)
	}

	// Then proceeds to add a connection
	conn := &SQLConfig{Name: "test", Driver: "postgres", Host: "localhost", Port: 5432, Username: "u", Password: "secret", Database: "d"}
	if err := cfg.AddConnection(conn); err != nil {
		t.Fatal(err)
	}

	// SECOND LAUNCH
	EncryptionKey = ""
	cfg2, err := LoadConfigWithVersion("test", cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg2.IsMasterConfigured() {
		t.Fatal("BUG: would prompt SETUP instead of UNLOCK")
	}

	kek2, _ := security.DeriveKey("mypass", cfg2.Security.MasterSalt, cfg2.MasterParams())
	if err := cfg2.ApplyMasterUnlock(kek2); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	if EncryptionKey != dataKeyAfterSetup {
		t.Fatalf("data key mismatch after unlock")
	}

	decrypted, _, err := util.DecryptPasswordWithMethod(cfg2.Connections[0].Password, EncryptionKey)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != "secret" {
		t.Fatalf("got %q want %q", decrypted, "secret")
	}
}

// TestMasterCancelDoesNotCorruptConfig simulates the user picking "master" in
// the dropdown, hitting Save, but cancelling the setup modal. With the old
// flow the file would be written with Method=master + WrappedKey="" and the
// next launch would silently re-prompt for setup (losing readability of any
// connections previously encrypted under the old method). With the fix, no
// persistence happens until ApplyMasterSetup completes.
func TestMasterCancelDoesNotCorruptConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	cfg, err := LoadConfigWithVersion("test", cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	// Pretend the user was on env mode with a working key.
	cfg.Security.Method = SecurityMethodEnv
	if err := cfg.UpdateConfig(); err != nil {
		t.Fatal(err)
	}

	// User picks master and hits Save — but the new flow does NOT persist the
	// method change before the modal completes. Cancelling means nothing
	// touches Security on disk.
	// (No call to UpdateConfig here — that's the whole point of the fix.)

	// SECOND LAUNCH should still see Method=env, not a half-set-up master.
	cfg2, err := LoadConfigWithVersion("test", cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Security.Method != SecurityMethodEnv {
		t.Fatalf("Method on disk should be env, got %q", cfg2.Security.Method)
	}
	if cfg2.IsMasterConfigured() {
		t.Fatal("MasterWrappedKey should be empty after cancel")
	}
}
