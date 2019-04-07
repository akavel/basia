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
		if err != nil {
			wn, werr := w.Write([]byte(s))
			n, err = n+wn, werr
		}
	}
	for _, attr := range m[""] {
		write(attr + "\n")
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
		// FIXME: verify that name has no '\n', etc.
		write("\nName: " + name + "\n")
		for _, attr := range m[name] {
			// TODO: handle advanced base64-encoded attributes correctly
			write(attr + "\n")
		}
		if err != nil {
			return
		}
	}
	return
}
