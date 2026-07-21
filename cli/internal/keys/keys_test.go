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

	if err := WriteEnvFile(path, anon, service); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	gotAnon, gotService, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile: %v", err)
	}
	if gotAnon != anon {
		t.Errorf("anon key = %q, want %q", gotAnon, anon)
	}
	if gotService != service {
		t.Errorf("service key = %q, want %q", gotService, service)
	}
}

func TestReadEnvFile_MissingRequiredKeysErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("WOTP_ANON_KEY=wotp_anon_abc\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if _, _, err := ReadEnvFile(path); err == nil {
		t.Fatal("expected an error when the service key is missing")
	}
}
