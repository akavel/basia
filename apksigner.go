// Copyright 2014-2019 apksigner Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/exp/errors/fmt"

	"go.mozilla.org/pkcs7"
)

var (
	infile   = flag.String("i", "unsigned.apk", "input unsigned zip `archive`")
	outfile  = flag.String("o", "signed.apk", "name of signed output zip `archive` to create")
	keyfile  = flag.String("k", "key.pk8", "private key for signing, in PKCS#8 format")
	certfile = flag.String("c", "key.x509.pem", "certificate for signing")
)

func main() {
	// USAGE: apksigner -i old.zip -o new-signed.zip
	flag.Parse()

	// Open signing key/cert files
	rawKey, err := ioutil.ReadFile(*keyfile)
	if err != nil {
		die(err)
	}
	key, err := x509.ParsePKCS8PrivateKey(rawKey)
	if err != nil {
		die(fmt.Errorf("parsing PKCS8: %s: %w", *keyfile, err))
	}
	certPEM, err := ioutil.ReadFile(*certfile)
	if err != nil {
		die(err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if x509.IsEncryptedPEMBlock(certBlock) {
		die(fmt.Errorf("%s: encrypted certificates currently not supported", *certfile))
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		die(err)
	}

	// Open infile & outfile zips
	zr, err := zip.OpenReader(*infile)
	if err != nil {
		die(err)
	}
	defer zr.Close()
	w, err := os.Create(*outfile)
	if err != nil {
		die(err)
	}
	defer func() {
		err := w.Close()
		if err != nil {
			die(err)
		}
	}()
	zw := zip.NewWriter(w)
	defer func() {
		err := zw.Close()
		if err != nil {
			die(err)
		}
	}()

	// TODO(akavel): normalize paths in r

	err = signZip(&zr.Reader, zw, cert, key)
	if err != nil {
		die(err)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

const (
	pathManifest = "META-INF/MANIFEST.MF"
	pathCertSf   = "META-INF/CERT.SF"
	pathCertRsa  = "META-INF/CERT.RSA"
)

func signZip(r *zip.Reader, w *zip.Writer, cert *x509.Certificate, privkey crypto.PrivateKey) error {
	// Copy main section of manifest from old zip, or create new one if absent.
	oldManifest, err := getOrInitManifest(r)
	if err != nil {
		return err
	}
	manifest := Manifest{"": oldManifest[""]}

	// Calculate digests of all files in the zip (sorted, for determinism),
	// adding them to the manifest.
	sort.Slice(r.File, func(i, j int) bool {
		return r.File[i].Name < r.File[j].Name
	})
	for _, f := range r.File {
		if f.FileInfo().IsDir() || isSpecialIgnored(f.Name) {
			continue
		}
		contents, err := f.Open()
		if err != nil {
			return err
		}
		hash, err := sha1sum(contents)
		if err != nil {
			return err
		}
		manifest[f.Name] = append(
			oldManifest[f.Name].Without("SHA1-Digest"),
			"SHA1-Digest: "+base64enc(hash[:]))
	}
	// Write the manifest file to the output zip archive.
	packed, err := w.Create(pathManifest)
	if err != nil {
		return err
	}
	_, err = manifest.WriteTo(packed)
	if err != nil {
		return err
	}

	// Generate signature file, and prepare it for signing
	buf := bytes.NewBuffer(nil)
	packed, err = w.Create(pathCertSf)
	if err != nil {
		return err
	}
	err = writeSignatureFile(io.MultiWriter(packed, buf), manifest, r.File)
	if err != nil {
		return err
	}

	// Sign the signature file
	sign, err := pkcs7.NewSignedData(buf.Bytes())
	if err != nil {
		return err
	}
	err = sign.AddSigner(cert, privkey, pkcs7.SignerInfoConfig{})
	if err != nil {
		return err
	}
	sign.Detach()
	signature, err := sign.Finish()
	if err != nil {
		return err
	}
	switch privkey.(type) {
	case *ecdsa.PrivateKey:
		packed, err = w.Create("META-INF/CERT.EC")
	case *rsa.PrivateKey:
		packed, err = w.Create("META-INF/CERT.RSA")
	default:
		return fmt.Errorf("TODO: unhandled type of private key: %T", privkey)
	}
	if err != nil {
		return err
	}
	_, err = packed.Write(signature)
	if err != nil {
		return err
	}

	// Copy all remaining files
	for _, f := range r.File {
		if f.FileInfo().IsDir() || isSpecialIgnored(f.Name) {
			continue
		}
		packed, err = w.CreateHeader(&zip.FileHeader{
			Name:    f.Name,
			Method:  zip.Deflate,
			Comment: f.Comment,
			Extra:   f.Extra,
			// Below fields are used to represent filesystem attributes, e.g. executable bit
			CreatorVersion: f.CreatorVersion,
			ExternalAttrs:  f.ExternalAttrs,
			// TODO: do we also need .ReaderVersion and .Flags for some reason?
		})
		contents, err := f.Open()
		if err != nil {
			return err
		}
		_, err = io.Copy(packed, contents)
		if err != nil {
			return fmt.Errorf("cannot copy file %q to output archive: %w", f.Name, err)
		}
		contents.Close()
	}
	return nil
}

// getOrInitManifest returns a parsed META-INF/MANIFEST.MF file from r, or a
// new Manifest with initialized main section if not found.
func getOrInitManifest(r *zip.Reader) (Manifest, error) {
	rawManifest := zipFind(r, pathManifest)
	if rawManifest == nil {
		return Manifest{
			"": Attributes{
				"Manifest-Version: 1.0",
				"Created-By: 1.0 (Android SignApk)",
			},
		}, nil
	}
	fr, err := rawManifest.Open()
	if err != nil {
		return nil, err
	}
	defer fr.Close()
	return ParseManifest(fr)
}

// isSpecialIgnored returns true if name is one of the special paths that
// should not be taken into account when calculating a hash/signature of an
// .apk file. Coincidentally, those same files also should not be copied
// verbatim to the output .apk file.
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
	return name == pathManifest ||
		match("META-INF/*.SF", name) ||
		match("META-INF/*.RSA", name) ||
		match("META-INF/*.DSA", name) ||
		match("META-INF/*.EC", name) || // *.EC observed in ECDSA-signed .apk files
		match("META-INF/SIG-*", name)
}

func writeSignatureFile(w io.Writer, manifest Manifest, sortedFiles []*zip.File) (err error) {
	write := func(s string) {
		if err == nil {
			_, err = w.Write([]byte(s))
		}
	}
	write("Signature-Version: 1.0\r\n")
	write("Created-By: 1.0 (Android SignApk)\r\n")
	if err != nil {
		return
	}
	hasher := sha1.New()
	_, err = manifest.WriteTo(hasher)
	if err != nil {
		return
	}
	write("SHA1-Digest-Manifest: " + base64enc(hasher.Sum(nil)) + "\r\n\r\n")
	if err != nil {
		return
	}
	for _, f := range sortedFiles {
		if len(manifest[f.Name]) == 0 {
			continue
		}
		hasher := sha1.New()
		_, err = manifest.WriteEntry(hasher, f.Name)
		if err != nil {
			return
		}
		_, err = hasher.Write([]byte("\r\n"))
		if err != nil {
			return
		}
		write("Name: " + f.Name + "\r\n")
		write("SHA1-Digest: " + base64enc(hasher.Sum(nil)) + "\r\n\r\n")
		if err != nil {
			return
		}
	}
	return
}

// zipFind returns a file with specified name from r, or nil if not found.
func zipFind(r *zip.Reader, name string) *zip.File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}
	return nil
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
