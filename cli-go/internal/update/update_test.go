package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestChecksumAndExtraction(t *testing.T) {
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	tw := tar.NewWriter(z)
	content := []byte("binary")
	if err := tw.WriteHeader(&tar.Header{Name: "bwt", Mode: 0755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	tw.Write(content)
	tw.Close()
	z.Close()
	hash := sha256.Sum256(b.Bytes())
	checksums := []byte(fmt.Sprintf("%x  bing-webmaster-cli_darwin_arm64.tar.gz\n", hash))
	if err := VerifyChecksum(b.Bytes(), checksums, "bing-webmaster-cli_darwin_arm64.tar.gz"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksum([]byte("tampered"), checksums, "bing-webmaster-cli_darwin_arm64.tar.gz"); err == nil {
		t.Fatal("accepted tampered archive")
	}
	got, err := ExtractBinary(b.Bytes())
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("extract %s %v", got, err)
	}
}
