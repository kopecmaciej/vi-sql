package security

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/util"
)

func TestDeriveKeyDeterministic(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt: %v", err)
	}
	params := DefaultArgon2idParams()

	k1, err := DeriveKey("hunter2", salt, params)
	if err != nil {
		t.Fatalf("DeriveKey #1: %v", err)
	}
	k2, err := DeriveKey("hunter2", salt, params)
	if err != nil {
		t.Fatalf("DeriveKey #2: %v", err)
	}
	if k1 != k2 {
		t.Fatalf("expected deterministic output for same inputs, got %q vs %q", k1, k2)
	}
}

func TestDeriveKeyDifferentSaltsDiffer(t *testing.T) {
	s1, _ := GenerateSalt()
	s2, _ := GenerateSalt()
	params := DefaultArgon2idParams()

	k1, _ := DeriveKey("same-passphrase", s1, params)
	k2, _ := DeriveKey("same-passphrase", s2, params)
	if k1 == k2 {
		t.Fatalf("expected different keys for different salts, got identical %q", k1)
	}
}

func TestDeriveKeyAsyncDoesNotBlock(t *testing.T) {
	salt, _ := GenerateSalt()
	params := DefaultArgon2idParams()

	start := time.Now()
	ch := DeriveKeyAsync("hunter2", salt, params)
	dispatch := time.Since(start)

	// Caller should return long before Argon2 finishes. With default params on a
	// modern machine derivation runs in ~100-300ms; we give the dispatch a 20ms
	// budget which is still ~5x the realistic dispatch cost while staying well
	// under the work duration.
	if dispatch > 20*time.Millisecond {
		t.Fatalf("DeriveKeyAsync blocked for %v; expected <20ms dispatch", dispatch)
	}

	select {
	case res := <-ch:
		if res.Err != nil {
			t.Fatalf("derive error: %v", res.Err)
		}
		if res.Key == "" {
			t.Fatalf("expected non-empty key")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("DeriveKeyAsync did not deliver result within 5s")
	}
}

func TestWrapUnwrapRoundTrip(t *testing.T) {
	salt, _ := GenerateSalt()
	params := DefaultArgon2idParams()
	kek, err := DeriveKey("correct horse battery staple", salt, params)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	dataKey, err := util.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey: %v", err)
	}

	wrapped, err := util.EncryptPassword(dataKey, kek)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	unwrapped, err := util.DecryptPassword(wrapped, kek)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if unwrapped != dataKey {
		t.Fatalf("round-trip mismatch: got %q want %q", unwrapped, dataKey)
	}
}

func TestUnwrapWithWrongPasswordFails(t *testing.T) {
	salt, _ := GenerateSalt()
	params := DefaultArgon2idParams()
	kekRight, _ := DeriveKey("right", salt, params)
	kekWrong, _ := DeriveKey("wrong", salt, params)

	dataKey, _ := util.GenerateEncryptionKey()
	wrapped, _ := util.EncryptPassword(dataKey, kekRight)

	if _, err := util.DecryptPassword(wrapped, kekWrong); err == nil {
		t.Fatalf("expected unwrap with wrong KEK to fail, got success")
	}
}

func TestUnwrapWithSameSaltDifferentParamsFails(t *testing.T) {
	salt, _ := GenerateSalt()
	pA := DefaultArgon2idParams()
	pB := DefaultArgon2idParams()
	pB.Iterations = pA.Iterations + 1

	kA, _ := DeriveKey("hunter2", salt, pA)
	kB, _ := DeriveKey("hunter2", salt, pB)

	dataKey, _ := util.GenerateEncryptionKey()
	wrapped, _ := util.EncryptPassword(dataKey, kA)

	if _, err := util.DecryptPassword(wrapped, kB); err == nil {
		t.Fatalf("expected unwrap to fail when KDF params changed, got success")
	}
}
