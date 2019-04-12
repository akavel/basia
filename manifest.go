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
	"bufio"
	"bytes"
	"io"
	"sort"
	"strings"

	"golang.org/x/exp/errors/fmt"
)

// References about .jar/.apk manifest files:
// - https://docs.oracle.com/javase/7/docs/technotes/guides/jar/jar.html

type Manifest map[string]Attributes
type Attributes []string

func (as Attributes) Without(key string) Attributes {
	key = key + ": "
	for i, v := range as {
		if strings.HasPrefix(v, key) {
			return append(as[:i:i], as[i+1:]...)
		}
	}
	return as
}

func ParseManifest(r io.Reader) (Manifest, error) {
	const namePrefix = "Name: "
	m := Manifest{}
	k, v := "", Attributes{}
	// TODO: handle advanced base64-encoded attributes correctly
	scan := bufio.NewScanner(
		io.MultiReader(r, strings.NewReader("\r\n\r\n")))
	for scan.Scan() {
		line := scan.Text()
		switch {
		case line == "":
			// new section
			if len(v) > 0 {
				m[k] = v
				k, v = "", Attributes{}
			}
		case strings.HasPrefix(line, namePrefix):
			k = line[len(namePrefix):]
		case strings.HasPrefix(line, " "):
			if len(v) == 0 {
				k += line[1:]
			} else {
				// TODO: optimize (?)
				v[len(v)-1] += line[1:]
			}
		default:
			v = append(v, line)
		}
	}
	if scan.Err() != nil {
		return nil, fmt.Errorf("META-INF/MANIFEST.MF: %w", scan.Err())
	}
	return m, nil
}

func (m Manifest) WriteTo(w io.Writer) (n int64, err error) {
	w = &wrap72{Writer: w}
	write := func(s string) {
		if err == nil {
			wn, werr := w.Write([]byte(s))
			n, err = n+int64(wn), werr
		}
	}
	for _, attr := range m[""] {
		write(attr + "\r\n")
	}
	if err != nil {
		return
	}
	// Sort the manifest filenames
	names := []string{}
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	if names[0] == "" {
		names = names[1:]
	}
	// Print the sorted per-file sections of the manifest
	for _, name := range names {
		write("\r\n")
		wn, werr := m.WriteEntry(w, name)
		n, err = n+wn, werr
		if err != nil {
			return
		}
	}
	// HACK: for now, adding extra newline for easier comparison with one real
	// .apk file; try to remove it in future if OK
	write("\r\n")
	return
}

func (m Manifest) WriteEntry(w io.Writer, name string) (n int64, err error) {
	w = &wrap72{Writer: w}
	write := func(s string) {
		if err == nil {
			wn, werr := w.Write([]byte(s))
			n, err = n+int64(wn), werr
		}
	}
	// FIXME: verify that name has no '\n', etc.
	write("Name: " + name + "\r\n")
	for _, attr := range m[name] {
		// TODO: handle advanced base64-encoded attributes correctly
		write(attr + "\r\n")
	}
	return
}

// wrap72 writes to Writer, splitting any lines exceeding 72 bytes (including
// the terminating "\r\n"). Continuation of a split line is marked with a
// single space " " prefix.
type wrap72 struct {
	io.Writer
	n int
}

func (w *wrap72) Write(buf []byte) (n int, err error) {
	const max = 70
	for len(buf) > 0 {
		i := bytes.IndexAny(buf, "\r\n")
		if i == 0 {
			// Newline characters (CR/LF) are safe to write
			for i < len(buf) && (buf[i] == '\r' || buf[i] == '\n') {
				i++
			}
			wn, werr := w.Writer.Write(buf[:i])
			n += wn
			if werr != nil {
				return n, werr
			}
			w.n = 0
			buf = buf[i:]
			continue
		}
		if i == -1 {
			i = len(buf)
		}
		if w.n == max {
			_, werr := w.Writer.Write([]byte("\r\n "))
			if werr != nil {
				return n, werr
			}
			w.n = 1
		}
		// If line exceeds max length (not counting final CR/LF), split it
		if w.n+i > 70 {
			i = 70 - w.n
		}
		wn, werr := w.Writer.Write(buf[:i])
		n += wn
		if werr != nil {
			return n, werr
		}
		w.n += i
		buf = buf[i:]
	}
	return
}
