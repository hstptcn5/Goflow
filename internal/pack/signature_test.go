package pack

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundleSignatureValidAndTrustFailures(t *testing.T) {
	unsigned := signatureTestBundle(t)
	privatePEM, publicPEM := signatureTestKeys(t)
	signed := filepath.Join(t.TempDir(), "signed.zip")
	if err := SignBundleArchive(unsigned, signed, "publisher.test-2026", privatePEM); err != nil {
		t.Fatalf("SignBundleArchive: %v", err)
	}
	secondSigned := filepath.Join(t.TempDir(), "signed.zip")
	if err := SignBundleArchive(unsigned, secondSigned, "publisher.test-2026", privatePEM); err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := os.ReadFile(signed)
	secondBytes, _ := os.ReadFile(secondSigned)
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("signed container is not deterministic")
	}
	if err := SignBundleArchive(unsigned, signed, "publisher.test-2026", privatePEM); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing signed output was overwritten: %v", err)
	}
	if err := VerifyBundleSignature(signed, "publisher.test-2026", publicPEM); err != nil {
		t.Fatalf("VerifyBundleSignature: %v", err)
	}
	if state, err := BundleSignatureState(signed); err != nil || state != "SIGNED_UNVERIFIED" {
		t.Fatalf("signature state = %q, %v", state, err)
	}
	if err := VerifyBundleSignature(signed, "wrong-key-id", publicPEM); err == nil || !strings.Contains(err.Error(), "key_id") {
		t.Fatalf("wrong key ID accepted: %v", err)
	}
	_, otherPublic := signatureTestKeys(t)
	if err := VerifyBundleSignature(signed, "publisher.test-2026", otherPublic); err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("wrong key accepted: %v", err)
	}
	if err := VerifyBundleSignature(unsigned, "publisher.test-2026", publicPEM); err == nil || !strings.Contains(err.Error(), "unsigned") {
		t.Fatalf("unsigned bundle accepted: %v", err)
	}
	if err := SignBundleArchive(signed, filepath.Join(t.TempDir(), "twice.zip"), "publisher.test-2026", privatePEM); err == nil || !strings.Contains(err.Error(), "already contains") {
		t.Fatalf("signed bundle was signed twice: %v", err)
	}
}

func TestBundleSignatureRejectsMetadataAndPayloadTamper(t *testing.T) {
	unsigned := signatureTestBundle(t)
	privatePEM, publicPEM := signatureTestKeys(t)
	signed := filepath.Join(t.TempDir(), "signed.zip")
	if err := SignBundleArchive(unsigned, signed, "publisher.test-2026", privatePEM); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   string
	}{
		{name: "unknown algorithm", mutate: mutateSignatureJSON(t, func(s *Signature) { s.Algorithm = "future" }), want: "unsupported"},
		{name: "future schema", mutate: mutateSignatureJSON(t, func(s *Signature) { s.SchemaVersion = 2 }), want: "unsupported"},
		{name: "version mismatch", mutate: mutateSignatureJSON(t, func(s *Signature) { s.PackVersion = "9.9.9" }), want: "identity"},
		{name: "target mismatch", mutate: mutateSignatureJSON(t, func(s *Signature) {
			if s.Target == "windows-amd64" {
				s.Target = "linux-amd64"
			} else {
				s.Target = "windows-amd64"
			}
		}), want: "identity"},
		{name: "capability mismatch", mutate: mutateSignatureJSON(t, func(s *Signature) { s.RequiredCapabilities = nil }), want: "identity"},
		{name: "malformed signature", mutate: func([]byte) []byte { return []byte(`{"schema_version":`) }, want: "strict JSON"},
		{name: "unknown field", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte("\n}"), []byte(",\n  \"public_key\": \"forbidden\"\n}"), 1)
		}, want: "unknown field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := rewriteSignatureEntry(t, signed, tt.mutate, false, false)
			err := VerifyBundleSignature(path, "publisher.test-2026", publicPEM)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
	t.Run("oversized metadata", func(t *testing.T) {
		path := rewriteSignatureEntry(t, signed, func([]byte) []byte { return bytes.Repeat([]byte("x"), MaxSignatureBytes+1) }, false, false)
		if err := VerifyBundleSignature(path, "publisher.test-2026", publicPEM); err == nil || !strings.Contains(err.Error(), "byte limit") {
			t.Fatalf("oversized signature accepted: %v", err)
		}
	})
	t.Run("duplicate metadata", func(t *testing.T) {
		path := rewriteSignatureEntry(t, signed, func(data []byte) []byte { return data }, true, false)
		if err := VerifyBundleSignature(path, "publisher.test-2026", publicPEM); err == nil || !strings.Contains(err.Error(), "duplicate ZIP entry") {
			t.Fatalf("duplicate signature accepted: %v", err)
		}
	})
	t.Run("self inventory", func(t *testing.T) {
		path := rewriteZipMember(t, signed, "PACK_INFO.json", func(data []byte) []byte {
			var info PackInfo
			if err := json.Unmarshal(data, &info); err != nil {
				t.Fatal(err)
			}
			info.Files = append(info.Files, PackInfoFile{Path: SignatureFileName, SHA256: strings.Repeat("0", 64), Size: 1})
			output, err := json.Marshal(info)
			if err != nil {
				t.Fatal(err)
			}
			return output
		})
		if err := VerifyBundleSignature(path, "publisher.test-2026", publicPEM); err == nil || !strings.Contains(err.Error(), "must not be listed") {
			t.Fatalf("self-inventoried signature accepted: %v", err)
		}
	})
	t.Run("symlink metadata", func(t *testing.T) {
		path := rewriteSignatureEntry(t, signed, func(data []byte) []byte { return data }, false, true)
		if err := VerifyBundleSignature(path, "publisher.test-2026", publicPEM); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("symlink signature accepted: %v", err)
		}
	})
	for _, member := range []string{runtimeEntry(CurrentPlatform()), "pack/pack.json", "pack/workflows/main.json", "pack/assets/signed.txt", "PACK_INFO.json"} {
		t.Run("tamper "+member, func(t *testing.T) {
			path := rewriteZipMember(t, signed, member, func(data []byte) []byte { return append(data, 'x') })
			if err := VerifyBundleSignature(path, "publisher.test-2026", publicPEM); err == nil {
				t.Fatalf("payload tamper accepted")
			}
		})
	}
	t.Run("extracted verification", func(t *testing.T) {
		root := extractSignatureBundle(t, signed)
		if err := VerifyBundleSignature(root, "publisher.test-2026", publicPEM); err != nil {
			t.Fatalf("extracted signature failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "pack", "workflows", "main.json"), []byte("tampered"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := VerifyBundleSignature(root, "publisher.test-2026", publicPEM); err == nil {
			t.Fatal("extracted tamper accepted")
		}
	})
}

func TestCanonicalSignaturePayloadDeterministicAndSensitive(t *testing.T) {
	value := Signature{SchemaVersion: 1, Algorithm: SignatureAlgorithm, KeyID: "key", PackID: "example.pack", PackVersion: "1.2.3", Target: "windows-amd64", RequiredCapabilities: []string{CapabilityPackV1}, PackInfoSHA256: strings.Repeat("ab", 32)}
	one := canonicalSignaturePayload(value)
	two := canonicalSignaturePayload(value)
	if !bytes.Equal(one, two) {
		t.Fatal("canonical payload is not deterministic")
	}
	value.Target = "linux-amd64"
	if bytes.Equal(one, canonicalSignaturePayload(value)) {
		t.Fatal("canonical payload did not bind target")
	}
}

func signatureTestBundle(t *testing.T) string {
	t.Helper()
	dir := writeBuildPack(t, func(m *Manifest) {
		m.RequiredCapabilities = []string{CapabilityPackV1}
		m.Assets = []string{"assets/signed.txt"}
	})
	writeFile(t, filepath.Join(dir, "assets", "signed.txt"), "signed asset")
	return mustBuild(t, dir, writeRuntimeFixture(t, "runtime-signature-canary"), t.TempDir()).ArchivePath
}

func extractSignatureBundle(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		path := filepath.Join(root, filepath.FromSlash(file.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		output, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			input.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(output, input)
		input.Close()
		output.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
	}
	return root
}

func signatureTestKeys(t *testing.T) ([]byte, []byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
}

func mutateSignatureJSON(t *testing.T, mutate func(*Signature)) func([]byte) []byte {
	t.Helper()
	return func(data []byte) []byte {
		var value Signature
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		mutate(&value)
		output, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		return append(output, '\n')
	}
}

func rewriteSignatureEntry(t *testing.T, source string, mutate func([]byte) []byte, duplicate, symlink bool) string {
	t.Helper()
	return rewriteZip(t, source, func(name string, header *zip.FileHeader, data []byte) ([]byte, bool) {
		if name != SignatureFileName {
			return data, true
		}
		if symlink {
			header.SetMode(os.ModeSymlink | 0777)
		}
		return mutate(data), true
	}, duplicate)
}

func rewriteZipMember(t *testing.T, source, member string, mutate func([]byte) []byte) string {
	t.Helper()
	return rewriteZip(t, source, func(name string, _ *zip.FileHeader, data []byte) ([]byte, bool) {
		if name == member {
			return mutate(data), true
		}
		return data, true
	}, false)
}

func rewriteZip(t *testing.T, source string, mutate func(string, *zip.FileHeader, []byte) ([]byte, bool), duplicateSignature bool) string {
	t.Helper()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	path := filepath.Join(t.TempDir(), "mutated.zip")
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	var signatureData []byte
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(input)
		input.Close()
		if err != nil {
			t.Fatal(err)
		}
		header := file.FileHeader
		data, keep := mutate(file.Name, &header, data)
		if !keep {
			continue
		}
		destination, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := destination.Write(data); err != nil {
			t.Fatal(err)
		}
		if file.Name == SignatureFileName {
			signatureData = data
		}
	}
	if duplicateSignature {
		header := &zip.FileHeader{Name: SignatureFileName, Method: zip.Deflate, Modified: zipTimestamp}
		destination, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := destination.Write(signatureData); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
