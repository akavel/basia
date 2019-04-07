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

	err = signZip(&zr.Reader, zw)
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

func signZip(r *zip.Reader, w *zip.Writer) error {
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
		if f.FileInfo().IsDir() || oneOf(f.Name, pathManifest, pathCertSf, pathCertRsa) {
			// TODO: also ignore META-INF/{*.SF,*.DSA,*.RSA,SIG-*} per below link ?
			// https://docs.oracle.com/javase/7/docs/technotes/guides/jar/jar.html#Signed_JAR_File
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

	// Generate signature file
	packed, err = w.Create(pathCertSf)
	if err != nil {
		return err
	}
	err = writeSignatureFile(packed, manifest, r.File)
	if err != nil {
		return err
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

// oneOf returns true if needle is equal to one of the strings from haystack.
func oneOf(needle string, haystack ...string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
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
