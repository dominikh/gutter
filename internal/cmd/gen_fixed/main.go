// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"strconv"
	"text/template"
)

func wrongArgs() {
	fmt.Fprintln(os.Stderr, "Usage: gen_fixed <signed|unsigned> <integer bits> <fractional bits>")
	os.Exit(1)
}

func main() {
	if len(os.Args) != 4 {
		wrongArgs()
	}

	var signed bool
	switch os.Args[1] {
	case "signed":
		signed = true
	case "unsigned":
		signed = false
	default:
		wrongArgs()
	}
	intBits, err := strconv.Atoi(os.Args[2])
	if err != nil {
		wrongArgs()
	}
	fracBits, err := strconv.Atoi(os.Args[3])
	if err != nil {
		wrongArgs()
	}

	switch intBits + fracBits {
	case 64, 32, 16, 8:
	default:
		fmt.Fprintln(os.Stderr, "combined number of bits must be a standard integer size")
		os.Exit(2)
	}

	basicType := fmt.Sprintf("uint%d", intBits+fracBits)
	var typeName string
	if signed {
		typeName = fmt.Sprintf("Int%d_%d", intBits, fracBits)
	} else {
		typeName = fmt.Sprintf("Uint%d_%d", intBits, fracBits)
	}

	tmpl := `
type {{ .TypeName }} {{ .BasicType }}

func (v {{ .TypeName }}) Float() float64 { return float64(v.Integer()) + float64(v.Fraction())/float64(1<<{{ .FracBits }}) }
func (v {{ .TypeName }}) Integer() int {
{{- if .Signed }} return int(v>>{{ .FracBits }}) ^ 1<<({{ .IntBits }}-1) - 1<<({{ .IntBits }}-1)
{{-  else }} return int(v >> {{ .FracBits }})
{{- end }} }
func (v {{ .TypeName }}) Fraction() int { return int(v & ((1 << {{ .FracBits }}) - 1)) }
func (v {{ .TypeName }}) String() string { return fmt.Sprintf("%d+%d/%d", v.Integer(), v.Fraction(), 1<<{{ .FracBits }}) }
`
	tpl := template.Must(template.New("").Parse(tmpl))
	tpl.Execute(os.Stdout, map[string]any{
		"TypeName":  typeName,
		"BasicType": basicType,
		"FracBits":  fracBits,
		"IntBits":   intBits,
		"Signed":    signed,
	})
}
