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
	"io"
	"sort"
	"strings"

	"golang.org/x/exp/errors/fmt"
)

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
	scan := bufio.NewScanner(r)
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
	return
}

func (m Manifest) WriteEntry(w io.Writer, name string) (n int64, err error) {
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
