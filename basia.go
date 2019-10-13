package main

import (
	"archive/zip"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.mozilla.org/pkcs7"
)

var (
	input    = flag.String("i", "", "path to `directory` containing files to put in an .apk")
	output   = flag.String("o", "", "path to `.apk` file to create")
	certfile = flag.String("c", "cert.x509.pem", "certificate for signing")
	keyfile  = flag.String("k", "key.pk8", "private key for signing, in PKCS#8 format")
)

func main() {
	// TODO: usage info
	flag.Parse()

	cert, key, err := loadCertAndKey(*certfile, *keyfile)
	check(err)

	// Open output .zip - early, to quickly verify if we have write permissions
	w, err := os.Create(*output)
	check(err)
	defer func() { check(w.Close()) }()
	zw := zip.NewWriter(w)
	defer func() { check(zw.Close()) }()

	// Collect names & hashes of files from input directory
	type file struct{ name, data string }
	files := []file{}
	check(filepath.Walk(*input, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relpath := filepath.ToSlash(strings.TrimLeft(strings.TrimPrefix(path, *input), `/\`))
		fmt.Println("#", relpath)
		switch relpath {
		case "META-INF/MANIFEST.MF", "meta-inf/manifest.mf":
			die(fmt.Errorf("modifying existing META-INF/MANIFEST.MF file not yet implemented"))
		}
		f, err := os.Open(path)
		check(err)
		hash, err := sha1sum(f)
		check(err)
		f.Close()
		files = append(files, file{relpath, base64enc(hash[:])})
		return nil
	}))
	sort.Slice(files, func(i, j int) bool {
		return files[i].name < files[j].name
	})

	// Build MANIFEST.MF
	manifestMf := joinBlock(
		"Manifest-Version: 1.0",
		"Built-By: Generated-by-ADT",
		"Created-By: Android Gradle 3.3.2")
	for i, f := range files {
		if isSpecialIgnored(f.name) {
			continue
		}
		entry := joinBlock(
			"Name: "+f.name,
			// Note: using SHA1 (not SHA256) to support old Android devices (https://stackoverflow.com/a/34875983/98528)
			"SHA1-Digest: "+f.data)
		manifestMf += entry
		files[i].data = entry // will be needed in CERT.SF
	}

	// Build CERT.SF
	certSf := joinBlock(
		"Signature-Version: 1.0",
		"Created-By: 1.0 (Android)",
		"SHA1-Digest-Manifest: "+base64sha1(manifestMf))
	for _, f := range files {
		if isSpecialIgnored(f.name) {
			continue
		}
		certSf += joinBlock(
			"Name: "+f.name,
			"SHA1-Digest: "+base64sha1(f.data))
	}

	// Calculate CERT.RSA or CERT.EC
	signed, err := sign([]byte(certSf), cert, key)
	check(err)
	signedName := ""
	switch key.(type) {
	case *ecdsa.PrivateKey:
		signedName = "META-INF/CERT.EC"
	case *rsa.PrivateKey:
		signedName = "META-INF/CERT.RSA"
	default:
		die(fmt.Errorf("TODO: unhandled type of private key: %T", key))
	}

	// Write result
	for _, f := range []file{
		{"META-INF/MANIFEST.MF", manifestMf},
		{"META-INF/CERT.SF", certSf},
		{signedName, string(signed)}} {
		fmt.Println("+", f.name)
		fh, err := zw.Create(f.name)
		check(err)
		_, err = fh.Write([]byte(f.data))
		check(err)
	}
	for _, f := range files {
		fmt.Println("+", f.name)
		fh, err := os.Open(filepath.Join(*input, f.name))
		check(err)
		fi, err := fh.Stat()
		check(err)
		zi, err := zip.FileInfoHeader(fi)
		check(err)
		zi.Name = f.name
		zi.Method = zip.Deflate
		zi.Modified, zi.ModifiedDate, zi.ModifiedTime = time.Time{}, 0, 0
		zh, err := zw.CreateHeader(zi)
		check(err)
		_, err = io.Copy(zh, fh)
		check(err)
		fh.Close()
	}
}

func loadCertAndKey(certfile, keyfile string) (*x509.Certificate, crypto.PrivateKey, error) {
	certPEM, err := ioutil.ReadFile(certfile)
	if err != nil {
		return nil, nil, err
	}
	certBlock, _ := pem.Decode(certPEM)
	if x509.IsEncryptedPEMBlock(certBlock) {
		return nil, nil, fmt.Errorf("%s: encrypted certificates currently not supported", certfile)
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", certfile, err)
	}

	rawKey, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(rawKey)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", keyfile, err)
		// die(fmt.Errorf("parsing PKCS8: %s: %w", keyfile, err))
	}

	return cert, key, nil
}

func joinBlock(lines ...string) (block string) {
	for _, l := range lines {
		block += wrap70(l) + "\r\n"
	}
	block += "\r\n"
	return
}
func wrap70(s string) (wrapped string) {
	max := 70
	for len(s) > max {
		wrapped += s[:max] + "\r\n "
		s = s[max:]
		max = 69
	}
	wrapped += s
	return
}

func isSpecialIgnored(name string) bool {
	if !strings.HasPrefix(name, "META-INF/") {
		return false // small optimization
	}
	match := func(pattern, name string) bool {
		m, err := path.Match(pattern, name)
		if err != nil {
			panic(err)
		}
		return m
	}
	// https://docs.oracle.com/javase/7/docs/technotes/guides/jar/jar.html#Signed_JAR_File
	return name == "META-INF/MANIFEST.MF" ||
		match("META-INF/*.SF", name) ||
		match("META-INF/*.RSA", name) ||
		match("META-INF/*.DSA", name) ||
		match("META-INF/*.EC", name) || // *.EC observed in ECDSA-signed .apk files
		match("META-INF/SIG-*", name)
}

func sign(data []byte, cert *x509.Certificate, privkey crypto.PrivateKey) ([]byte, error) {
	algo, err := pkcs7.NewSignedData(data)
	if err != nil {
		return nil, err
	}
	err = algo.AddSigner(cert, privkey, pkcs7.SignerInfoConfig{})
	if err != nil {
		return nil, err
	}
	algo.Detach()
	signature, err := algo.Finish()
	if err != nil {
		return nil, err
	}
	return signature, err
}

func base64sha1(s string) string {
	hash, _ := sha1sum(strings.NewReader(s))
	return base64enc(hash[:])
}

func sha1sum(r io.Reader) (sum [sha1.Size]byte, err error) {
	calc := sha1.New()
	_, err = io.Copy(calc, r)
	calc.Sum(sum[:0])
	return
}

func base64enc(buf []byte) string {
	return base64.StdEncoding.EncodeToString(buf)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
