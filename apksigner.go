package main

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

var (
	infile  = flag.String("i", "", "input unsigned zip `archive`")
	outfile = flag.String("o", "", "name of signed output zip `archive` to create")
)

func main() {
	// USAGE: apksigner -i old.zip -o new-signed.zip
	flag.Parse()

	r, err := zip.OpenReader(infile)
	if err != nil {
		die(err)
	}
	defer r.Close()
	w, err := os.Create(outfile)
	if err != nil {
		die(err)
	}
	defer w.Close()

	// TODO(akavel): normalize paths in r
	err = signZip(r, w)
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

func signZip(r zip.Reader, w zip.Writer) error {
	// Copy main section of manifest from old zip, or create new one.
	var oldManifest Manifest
	manifest := Manifest{
		"": Attributes{
			"Manifest-Version: 1.0",
			"Created-By: 1.0 (Android SignApk)",
		},
	}
	if rawManifest := zipFind(r, pathManifest); rawManifest != nil {
		oldManifest = ParseManifest(rawManifest)
		manifest[""] = oldManifest[""]
	}

	// Calculate digest of all files in the zip, sorted for determinism.
	sort.Sort(r.File, func(i, j int) bool {
		return r.File[i].Name < r.File[j].Name
	})
	for f := range r.File {
		if f.FileInfo().IsDir() || oneOf(f.Name, pathManifest, pathCertSf, pathCertRsa) {
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
		manifest[f.Name] = append(oldManifest[f.Name],
			"SHA1-Digest: "+base64.StdEncoding.EncodeToString(hash[:]))
	}
}

func zipFind(r zip.Reader, name string) *zip.File {
	for _, f := range r.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func oneOf(needle string, haystack ...string) bool {
	for s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func sha1sum(r io.Reader) (sum [sha1.Size]byte, err error) {
	calc := sha1.New()
	err = io.Copy(calc, r)
	calc.Sum(sum[:0])
}
