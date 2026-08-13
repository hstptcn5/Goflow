package pack

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	SignatureFileName      = "PACK_SIGNATURE.json"
	SignatureSchemaVersion = 1
	SignatureAlgorithm     = "ed25519"
	MaxSignatureBytes      = 16 << 10
	MaxKeyIDLength         = 128
	MaxKeyFileBytes        = 64 << 10
)

func BundleSignatureState(path string) (string, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		signaturePath := filepath.Join(path, SignatureFileName)
		entry, err := os.Lstat(signaturePath)
		if os.IsNotExist(err) {
			return "UNSIGNED", nil
		}
		if err != nil {
			return "", err
		}
		if entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
			return "", fmt.Errorf("signature metadata must be a regular file")
		}
		return "SIGNED_UNVERIFIED", nil
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	count := 0
	for _, file := range reader.File {
		if file.Name == SignatureFileName {
			count++
		}
	}
	if count == 0 {
		return "UNSIGNED", nil
	}
	if count != 1 {
		return "", fmt.Errorf("%s appears multiple times", SignatureFileName)
	}
	return "SIGNED_UNVERIFIED", nil
}

type Signature struct {
	SchemaVersion        int      `json:"schema_version"`
	Algorithm            string   `json:"algorithm"`
	KeyID                string   `json:"key_id"`
	PackID               string   `json:"pack_id"`
	PackVersion          string   `json:"pack_version"`
	Target               string   `json:"target"`
	RequiredCapabilities []string `json:"required_capabilities"`
	PackInfoSHA256       string   `json:"pack_info_sha256"`
	Signature            string   `json:"signature"`
}

func SignBundleArchive(inputPath, outputPath, keyID string, privateKeyData []byte) error {
	if inputPath == outputPath {
		return fmt.Errorf("signature output must differ from input archive")
	}
	if err := VerifyBundleArchiveFile(inputPath); err != nil {
		return err
	}
	privateKey, err := ParseEd25519PrivateKey(privateKeyData)
	if err != nil {
		return err
	}
	reader, infoFile, infoData, err := openUnsignedBundle(inputPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	var info PackInfo
	if err := json.Unmarshal(infoData, &info); err != nil {
		return fmt.Errorf("PACK_INFO.json is malformed: %w", err)
	}
	sum := sha256.Sum256(infoData)
	metadata := Signature{SchemaVersion: SignatureSchemaVersion, Algorithm: SignatureAlgorithm, KeyID: keyID, PackID: info.PackID, PackVersion: info.PackVersion, Target: info.Target, RequiredCapabilities: append([]string(nil), info.RequiredCapabilities...), PackInfoSHA256: hex.EncodeToString(sum[:])}
	if err := validateSignatureMetadata(metadata, false); err != nil {
		return err
	}
	metadata.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, canonicalSignaturePayload(metadata)))
	signatureData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	signatureData = append(signatureData, '\n')
	if len(signatureData) > MaxSignatureBytes {
		return fmt.Errorf("signature metadata exceeds %d byte limit", MaxSignatureBytes)
	}
	if err := writeSignedArchive(outputPath, reader, infoFile, signatureData); err != nil {
		return err
	}
	return verifyBundleSignatureKey(outputPath, keyID, privateKey.Public().(ed25519.PublicKey))
}

func VerifyBundleSignature(path, trustedKeyID string, publicKeyData []byte) error {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		if _, err := VerifyExtractedBundle(path); err != nil {
			return err
		}
	} else if err := VerifyBundleArchiveFile(path); err != nil {
		return err
	}
	publicKey, err := ParseEd25519PublicKey(publicKeyData)
	if err != nil {
		return err
	}
	return verifyBundleSignatureKey(path, trustedKeyID, publicKey)
}

func verifyBundleSignatureKey(path, trustedKeyID string, publicKey ed25519.PublicKey) error {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return verifyExtractedSignatureKey(path, trustedKeyID, publicKey)
	}
	if err := VerifyBundleArchiveFile(path); err != nil {
		return err
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	var signatureFile, infoFile *zip.File
	for _, file := range reader.File {
		switch file.Name {
		case SignatureFileName:
			if signatureFile != nil {
				return fmt.Errorf("%s appears multiple times", SignatureFileName)
			}
			signatureFile = file
		case "PACK_INFO.json":
			infoFile = file
		}
	}
	if signatureFile == nil {
		return fmt.Errorf("bundle is unsigned")
	}
	if signatureFile.FileInfo().Mode()&os.ModeSymlink != 0 || !signatureFile.Mode().IsRegular() {
		return fmt.Errorf("signature metadata must be a regular file")
	}
	signatureData, _, err := readZipFileLimited(signatureFile, MaxSignatureBytes)
	if err != nil {
		return fmt.Errorf("read signature metadata: %w", err)
	}
	metadata, err := decodeSignature(signatureData)
	if err != nil {
		return err
	}
	if metadata.KeyID != trustedKeyID {
		return fmt.Errorf("signature key_id %q does not match trusted key ID", metadata.KeyID)
	}
	infoData, _, err := readZipFileLimited(infoFile, MaxPackInfoBytes)
	if err != nil {
		return err
	}
	var info PackInfo
	if err := json.Unmarshal(infoData, &info); err != nil {
		return err
	}
	sum := sha256.Sum256(infoData)
	if metadata.PackInfoSHA256 != hex.EncodeToString(sum[:]) || metadata.PackID != info.PackID || metadata.PackVersion != info.PackVersion || metadata.Target != info.Target || !equalStringSlices(metadata.RequiredCapabilities, info.RequiredCapabilities) {
		return fmt.Errorf("signature metadata does not match PACK_INFO identity")
	}
	signature, err := base64.StdEncoding.DecodeString(metadata.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("signature value must be a base64 Ed25519 signature")
	}
	if !ed25519.Verify(publicKey, canonicalSignaturePayload(metadata), signature) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func verifyExtractedSignatureKey(root, trustedKeyID string, publicKey ed25519.PublicKey) error {
	info, err := VerifyExtractedBundle(root)
	if err != nil {
		return err
	}
	signaturePath := filepath.Join(root, SignatureFileName)
	entry, err := os.Lstat(signaturePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("bundle is unsigned")
	}
	if err != nil {
		return err
	}
	if entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
		return fmt.Errorf("signature metadata must be a regular file")
	}
	signatureData, _, err := readFileLimited(signaturePath, MaxSignatureBytes)
	if err != nil {
		return fmt.Errorf("read signature metadata: %w", err)
	}
	metadata, err := decodeSignature(signatureData)
	if err != nil {
		return err
	}
	if metadata.KeyID != trustedKeyID {
		return fmt.Errorf("signature key_id %q does not match trusted key ID", metadata.KeyID)
	}
	infoData, _, err := readFileLimited(filepath.Join(root, "PACK_INFO.json"), MaxPackInfoBytes)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(infoData)
	if metadata.PackInfoSHA256 != hex.EncodeToString(sum[:]) || metadata.PackID != info.PackID || metadata.PackVersion != info.PackVersion || metadata.Target != info.Target || !equalStringSlices(metadata.RequiredCapabilities, info.RequiredCapabilities) {
		return fmt.Errorf("signature metadata does not match PACK_INFO identity")
	}
	signature, err := base64.StdEncoding.DecodeString(metadata.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("signature value must be a base64 Ed25519 signature")
	}
	if !ed25519.Verify(publicKey, canonicalSignaturePayload(metadata), signature) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func ParseEd25519PrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, rest := pem.Decode(bytes.TrimSpace(data))
	if block == nil || len(bytes.TrimSpace(rest)) != 0 || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("private key must be one PEM PKCS#8 Ed25519 key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key must use Ed25519")
	}
	return edKey, nil
}

func ParseEd25519PublicKey(data []byte) (ed25519.PublicKey, error) {
	block, rest := pem.Decode(bytes.TrimSpace(data))
	if block == nil || len(bytes.TrimSpace(rest)) != 0 || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("public key must be one PEM PKIX Ed25519 key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	edKey, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key must use Ed25519")
	}
	return edKey, nil
}

func decodeSignature(data []byte) (Signature, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var metadata Signature
	if err := decoder.Decode(&metadata); err != nil {
		return Signature{}, fmt.Errorf("signature metadata must be strict JSON: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Signature{}, fmt.Errorf("signature metadata must contain one JSON object: %w", err)
	}
	if err := validateSignatureMetadata(metadata, true); err != nil {
		return Signature{}, err
	}
	return metadata, nil
}

func validateSignatureMetadata(value Signature, requireSignature bool) error {
	if value.SchemaVersion != SignatureSchemaVersion || value.Algorithm != SignatureAlgorithm {
		return fmt.Errorf("unsupported signature schema or algorithm")
	}
	if value.KeyID == "" || len(value.KeyID) > MaxKeyIDLength || strings.ContainsAny(value.KeyID, "\r\n\x00") {
		return fmt.Errorf("signature key_id is invalid")
	}
	if !isValidID(value.PackID) || !isValidSemVer(value.PackVersion) {
		return fmt.Errorf("signature Pack identity is invalid")
	}
	if _, ok := supportedPlatforms[value.Target]; !ok {
		return fmt.Errorf("signature target is unsupported")
	}
	seen := map[string]bool{}
	for _, capability := range value.RequiredCapabilities {
		if _, ok := supportedCapabilities[capability]; !ok || seen[capability] {
			return fmt.Errorf("signature capabilities are invalid")
		}
		seen[capability] = true
	}
	decoded, err := hex.DecodeString(value.PackInfoSHA256)
	if err != nil || len(decoded) != sha256.Size || value.PackInfoSHA256 != strings.ToLower(value.PackInfoSHA256) {
		return fmt.Errorf("signature PACK_INFO digest is invalid")
	}
	if requireSignature && value.Signature == "" {
		return fmt.Errorf("signature value is required")
	}
	return nil
}

func canonicalSignaturePayload(value Signature) []byte {
	var out bytes.Buffer
	out.WriteString("goflow-pack-signature-v1\n")
	_ = binary.Write(&out, binary.BigEndian, uint64(value.SchemaVersion))
	writeCanonicalString(&out, value.Algorithm)
	writeCanonicalString(&out, value.KeyID)
	writeCanonicalString(&out, value.PackID)
	writeCanonicalString(&out, value.PackVersion)
	writeCanonicalString(&out, value.Target)
	_ = binary.Write(&out, binary.BigEndian, uint64(len(value.RequiredCapabilities)))
	for _, capability := range value.RequiredCapabilities {
		writeCanonicalString(&out, capability)
	}
	digest, _ := hex.DecodeString(value.PackInfoSHA256)
	out.Write(digest)
	return out.Bytes()
}

func writeCanonicalString(out *bytes.Buffer, value string) {
	_ = binary.Write(out, binary.BigEndian, uint64(len(value)))
	out.WriteString(value)
}

func openUnsignedBundle(path string) (*zip.ReadCloser, *zip.File, []byte, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, nil, err
	}
	var infoFile *zip.File
	for _, file := range reader.File {
		if file.Name == SignatureFileName {
			reader.Close()
			return nil, nil, nil, fmt.Errorf("bundle already contains a signature")
		}
		if file.Name == "PACK_INFO.json" {
			infoFile = file
		}
	}
	if infoFile == nil {
		reader.Close()
		return nil, nil, nil, fmt.Errorf("PACK_INFO.json is missing")
	}
	data, _, err := readZipFileLimited(infoFile, MaxPackInfoBytes)
	if err != nil {
		reader.Close()
		return nil, nil, nil, err
	}
	return reader, infoFile, data, nil
}

func writeSignedArchive(outputPath string, reader *zip.ReadCloser, _ *zip.File, signatureData []byte) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(outputPath), ".signed-*.zip")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	writer := zip.NewWriter(temp)
	closed := false
	defer func() {
		if !closed {
			_ = writer.Close()
			_ = temp.Close()
		}
	}()
	for _, source := range reader.File {
		header := source.FileHeader
		header.Modified = zipTimestamp
		destination, err := writer.CreateHeader(&header)
		if err != nil {
			return err
		}
		input, err := source.Open()
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(destination, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	header := &zip.FileHeader{Name: SignatureFileName, Method: zip.Deflate, Modified: zipTimestamp}
	header.SetMode(0600)
	destination, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := destination.Write(signatureData); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	closed = true
	if err := os.Link(tempPath, outputPath); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("signature output already exists")
		}
		return fmt.Errorf("publish signed archive: %w", err)
	}
	return nil
}
