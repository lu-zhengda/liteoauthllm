package auth

import (
	"testing"
)

func TestGeneratePKCE(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("verifier length %d outside RFC 7636 range [43,128]", len(verifier))
	}

	if len(challenge) == 0 {
		t.Error("challenge should not be empty")
	}

	if verifier == challenge {
		t.Error("verifier and challenge should differ (S256)")
	}
}

func TestGeneratePKCEUniqueness(t *testing.T) {
	v1, _, _ := GeneratePKCE()
	v2, _, _ := GeneratePKCE()

	if v1 == v2 {
		t.Error("two PKCE generations should produce different verifiers")
	}
}
