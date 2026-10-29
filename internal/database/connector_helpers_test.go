package database

import "testing"

func TestResolvePasswordFields(t *testing.T) {
	t.Run("password and command are mutually exclusive", func(t *testing.T) {
		if _, _, err := ResolvePasswordFields("pw", "echo x", nil); err == nil {
			t.Error("expected error when both password and command are set")
		}
	})

	t.Run("command wins when only command is set", func(t *testing.T) {
		pw, cmd, err := ResolvePasswordFields("", "echo x", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pw != "" || cmd != "echo x" {
			t.Errorf("got pw=%q cmd=%q, want pw=\"\" cmd=%q", pw, cmd, "echo x")
		}
	})

	t.Run("password passes through when only password is set", func(t *testing.T) {
		pw, cmd, err := ResolvePasswordFields("secret", "", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pw != "secret" || cmd != "" {
			t.Errorf("got pw=%q cmd=%q, want pw=%q cmd=\"\"", pw, cmd, "secret")
		}
	})
}
