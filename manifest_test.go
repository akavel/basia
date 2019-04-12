package main

import (
	"strings"
	"testing"

	"github.com/kylelemons/godebug/pretty"
)

func TestParseManifest(test *testing.T) {
	cases := []struct {
		comment string
		input   string
		want    Manifest
	}{
		{
			comment: "multiline names",
			input: `Manifest-Version: 1.0
Built-By: Generated-by-ADT
Created-By: Android Gradle 3.3.2

Name: res/drawable/abc_list_selector_background_transition_holo_dark.x
 ml
SHA1-Digest: x6OHiSoyMWiuIOgpmUuAh/tRnYM=

Name: res/drawable/abc_list_selector_background_transition_holo_light.
 xml
SHA1-Digest: 0fvC1p6NZOpNNtjO4w0DBYRz8d0=
`,
			want: Manifest{
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
		have, err := ParseManifest(strings.NewReader(c.input))
		if err != nil {
			test.Errorf("%q: %s", c.comment, err)
			continue
		}
		if diff := pretty.Compare(have, c.want); diff != "" {
			test.Errorf("%q: diff (-have +want):\n%s", c.comment, diff)
		}
	}
}
