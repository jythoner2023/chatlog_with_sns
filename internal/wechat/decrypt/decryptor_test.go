package decrypt

import "testing"

func TestNewDecryptorSupportsDarwinV4(t *testing.T) {
	decryptor, err := NewDecryptor("darwin", 4)
	if err != nil {
		t.Fatalf("expected darwin v4 decryptor to be supported, got error: %v", err)
	}
	if decryptor == nil {
		t.Fatal("expected darwin v4 decryptor, got nil")
	}
}
