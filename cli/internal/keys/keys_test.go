package keys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKeyPair_Prefixes(t *testing.T) {
	anon, service, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if !strings.HasPrefix(anon, AnonPrefix) {
		t.Errorf("anon key %q missing prefix %q", anon, AnonPrefix)
	}
	if !strings.HasPrefix(service, ServicePrefix) {
		t.Errorf("service key %q missing prefix %q", service, ServicePrefix)
	}
}

func TestWriteReadEnvFile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	anon, service, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	root, err := GenerateKey(RootPrefix)
	if err != nil {
		t.Fatalf("GenerateKey(RootPrefix): %v", err)
	}

	if err := WriteEnvFile(path, anon, service, root); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	gotAnon, gotService, gotRoot, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if gotAnon != anon {
		t.Errorf("anon key = %q, want %q", gotAnon, anon)
	}
	if gotService != service {
		t.Errorf("service key = %q, want %q", gotService, service)
	}
	if gotRoot != root {
		t.Errorf("root key = %q, want %q", gotRoot, root)
	}
}

func TestReadEnvFile_MissingRootKeyIsTolerated(t *testing.T) {
	// .env files written before the root key existed shouldn't break reads —
	// callers that need the root key check for "" separately.
	path := filepath.Join(t.TempDir(), ".env")
	content := "WOTP_ANON_KEY=wotp_anon_abc\nWOTP_SERVICE_KEY=wotp_service_def\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	anon, service, root, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if anon != "wotp_anon_abc" || service != "wotp_service_def" {
		t.Fatalf("anon/service = %q/%q, want wotp_anon_abc/wotp_service_def", anon, service)
	}
	if root != "" {
		t.Fatalf("root = %q, want empty for a legacy .env file", root)
	}
}

func TestReadEnvFile_MissingRequiredKeysErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("WOTP_ANON_KEY=wotp_anon_abc\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if _, _, _, err := ReadEnvFile(path); err == nil {
		t.Fatal("expected an error when the service key is missing")
	}
}
