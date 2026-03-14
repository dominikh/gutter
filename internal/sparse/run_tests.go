//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
)

type result struct {
	goarch, goexp, tags, goamd64 string

	output []byte
	err    error
}

func run(goarch, goexp, tags, goamd64 string, extraArgs []string) result {
	args := []string{
		"test",
		"-tags", tags,
		"-short",
	}
	args = append(args, extraArgs...)
	cmd := exec.Command("go", args...)
	env := os.Environ()
	env = append(env,
		"GOEXPERIMENT="+goexp,
		"GOAMD64="+goexp,
		"GOARCH="+goarch,
	)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	return result{goarch, goexp, tags, goamd64, out, err}
}

func main() {
	tags := []string{"", "noasm"}
	exps := []string{"", "simd"}
	amd64s := []string{"v2", "v3"}
	archs := []string{"amd64", "386", "arm64"}

	var results []result
	for _, arch := range archs {
		for _, tag := range tags {
			for _, exp := range exps {
				if arch == "amd64" {
					for _, amd64 := range amd64s {
						results = append(results, run(arch, exp, tag, amd64, os.Args[1:]))
					}
				} else {
					results = append(results, run(arch, exp, tag, "", os.Args[1:]))
				}
			}
		}
	}

	failed := false
	for _, res := range results {
		if res.err == nil {
			continue
		}
		failed = true
		fmt.Fprintf(os.Stderr, "Test configuration GOEXPERIMENT=%s GOAMD64=%s GOARCH=%s -tags=%s failed:",
			res.goexp, res.goamd64, res.goarch, res.tags)
		fmt.Fprintf(os.Stderr, "Error: %s", res.err)
		fmt.Fprintf(os.Stderr, "Output:\n%s\n", res.output)
	}
	if failed {
		os.Exit(1)
	}
}
