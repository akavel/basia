package main

import (
	"bytes"
	"strings"
	"testing"

	differ "github.com/kylelemons/godebug/diff"
	"github.com/kylelemons/godebug/pretty"
)

func TestParseAndWriteManifest(test *testing.T) {
	cases := []struct {
		comment    string
		serialized string
		parsed     Manifest
	}{
		{
			comment: "multiline names",
			serialized: strings.ReplaceAll(`Manifest-Version: 1.0
Built-By: Generated-by-ADT
Created-By: Android Gradle 3.3.2

Name: res/drawable/abc_list_selector_background_transition_holo_dark.x
 ml
SHA1-Digest: x6OHiSoyMWiuIOgpmUuAh/tRnYM=

Name: res/drawable/abc_list_selector_background_transition_holo_light.
 xml
SHA1-Digest: 0fvC1p6NZOpNNtjO4w0DBYRz8d0=
`, "\n", "\r\n"),
			parsed: Manifest{
				"": Attributes{
					`Manifest-Version: 1.0`,
					`Built-By: Generated-by-ADT`,
					`Created-By: Android Gradle 3.3.2`,
				},
				"res/drawable/abc_list_selector_background_transition_holo_dark.xml": Attributes{
					`SHA1-Digest: x6OHiSoyMWiuIOgpmUuAh/tRnYM=`,
				},
				"res/drawable/abc_list_selector_background_transition_holo_light.xml": Attributes{
					`SHA1-Digest: 0fvC1p6NZOpNNtjO4w0DBYRz8d0=`,
				},
			},
		},
	}

	for _, c := range cases {
		parsed, err := ParseManifest(strings.NewReader(c.serialized))
		if err != nil {
			test.Errorf("%q: %s", c.comment, err)
		} else if diff := pretty.Compare(parsed, c.parsed); diff != "" {
			test.Errorf("%q: ParseManifest diff (-have +want):\n%s", c.comment, diff)
		}

		serialized := bytes.NewBuffer(nil)
		_, err = c.parsed.WriteTo(serialized)
		if err != nil {
			test.Errorf("%q: %s", c.comment, err)
		} else if diff := differ.Diff(serialized.String(), c.serialized); diff != "" {
			test.Errorf("%q: WriteTo diff (-have +want):\n%s", c.comment, diff)
		}
	}
}
