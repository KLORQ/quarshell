package main

import (
	"fmt"
	"github.com/KLORQ/quarshell-policy/corepolicy"
	"strings"
)

func main() {
	inputs := []struct{ name, body string }{
		{"over_upstream_4MiB", "version: 1\n#" + strings.Repeat("x", 4*1024*1024) + "\n"},
		{"null_process", "version: 1\nprocess: null\n"},
		{"null_network", "version: 1\nnetwork_policies: null\n"},
		{"null_filesystem", "version: 1\nfilesystem_policy: null\n"},
	}
	for _, input := range inputs {
		doc, err := policy.Parse([]byte(input.body))
		if err == nil {
			err = doc.Validate()
		}
		fmt.Printf("%s: bytes=%d error=%v\n", input.name, len(input.body), err)
	}
}
