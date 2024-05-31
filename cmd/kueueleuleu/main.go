// This file is part of kueueleuleu (https://github.com/norbjd/kueueleuleu).
//
// Copyright (C) 2023 norbjd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3 of the License.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/norbjd/kueueleuleu"
	"gopkg.in/yaml.v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

// these variables are filled via ldflags when building: e.g. go build -ldflags="-X main.version=0.0.1" [...].
//
//nolint:gochecknoglobals
var (
	version    = "unknown"
	commit     = "unknown"
	commitDate = "unknown"
	treeState  = "unknown"
)

var (
	errMalformedK8sObject    = errors.New("malformed k8s object")
	errUnknownK8sObject      = errors.New("unknown k8s object")
	errUnsupportedConversion = errors.New("unsupported conversion")
)

func main() {
	var (
		displayVersion bool
		help           bool
	)

	flag.BoolVar(&displayVersion, "version", false, "output version information and exit")
	flag.BoolVar(&help, "help", false, "display this help and exit")

	file := flag.String("f", "", "path to YAML file or - (stdin)")
	flag.Parse()

	if help {
		displayUsageAndExit(0)
	}

	if displayVersion {
		fmt.Fprintf(os.Stdout, `kueueleuleu %s (commit: %s, date: %s, tree state: %s)
Copyright © 2023 norbjd
License GPLv3: GNU GPL version 3 <https://gnu.org/licenses/gpl.html>
This is free software: you are free to change and redistribute it.
There is NO WARRANTY.
`, version, commit, commitDate, treeState)
		os.Exit(0)
	}

	if file == nil || *file == "" {
		log.Println("input is not set")
		displayUsageAndExit(1)
	}

	convertYAML(*file, os.Stdout)
}

func displayUsageAndExit(exitCode int) {
	flag.Usage()
	fmt.Fprintf(os.Stdout, `
Report bugs to: <https://github.com/norbjd/kueueleuleu/issues>
kueueleuleu home page: <https://github.com/norbjd/kueueleuleu>
`)
	os.Exit(exitCode)
}

func convertYAML(inputFilename string, w io.Writer) {
	input := getInput(inputFilename)
	convertReader(input, w)
}

func getInput(file string) io.Reader {
	var reader io.Reader

	switch file {
	case "-":
		reader = os.Stdin
	default:
		var err error
		reader, err = os.Open(file)

		if err != nil {
			log.Fatal(fmt.Errorf("cannot open file: %w", err))
		}
	}

	return reader
}

func convertReader(reader io.Reader, out io.Writer) {
	yamlDecoder := yaml.NewDecoder(reader)

	var k8sObject map[string]interface{}

	type meta struct {
		apiVersion string
		kind       string
	}

	for yamlDecoder.Decode(&k8sObject) == nil {
		apiVersion, isString := k8sObject["apiVersion"].(string)
		if !isString {
			log.Fatal(fmt.Errorf("%w: apiVersion is not a string", errMalformedK8sObject))
		}

		kind, isString := k8sObject["kind"].(string)
		if !isString {
			log.Fatal(fmt.Errorf("%w: kind is not a string", errMalformedK8sObject))
		}

		metaToConvert := meta{
			apiVersion: apiVersion,
			kind:       kind,
		}

		var outputYAML []byte

		var err error

		switch metaToConvert {
		case meta{apiVersion: "v1", kind: "Pod"}:
			outputYAML, err = convertToYAML(k8sObject, &corev1.Pod{})
		case meta{apiVersion: "batch/v1", kind: "Job"}:
			outputYAML, err = convertToYAML(k8sObject, &batchv1.Job{})
		case meta{apiVersion: "batch/v1", kind: "CronJob"}:
			outputYAML, err = convertToYAML(k8sObject, &batchv1.CronJob{})
		default:
			err = fmt.Errorf("%w: (%v, %v)", errUnknownK8sObject, metaToConvert.apiVersion, metaToConvert.kind)
		}

		if err != nil {
			log.Fatal(err)
		}

		_, err = out.Write(append([]byte("---\n"), outputYAML...))
		if err != nil {
			log.Fatal(err)
		}
	}
}

func convertToYAML(k8sObject map[string]interface{}, typedK8sObject metav1.Common) ([]byte, error) {
	kueueleuleuObject, err := convert(k8sObject, typedK8sObject)
	if err != nil {
		return nil, fmt.Errorf("cannot convert to kueueleuleu: %w", err)
	}

	outputYAML, err := kyaml.Marshal(kueueleuleuObject)
	if err != nil {
		return nil, fmt.Errorf("cannot read YAML: %w", err)
	}

	return outputYAML, nil
}

func convert(k8sObject map[string]interface{}, typedK8sObject metav1.Common) (metav1.Common, error) {
	k8sObjectYAML, err := kyaml.Marshal(k8sObject)
	if err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}

	err = kyaml.Unmarshal(k8sObjectYAML, &typedK8sObject)
	if err != nil {
		return nil, fmt.Errorf("cannot read YAML: %w", err)
	}

	converted, err := convertWithRightMethod(typedK8sObject)
	if err != nil {
		return nil, fmt.Errorf("cannot convert k8s object: %w", err)
	}

	return converted, err
}

func convertWithRightMethod(t metav1.Common) (metav1.Common, error) {
	switch tTyped := t.(type) {
	case *corev1.Pod:
		c, err := kueueleuleu.ConvertPod(*tTyped)
		if err != nil {
			err = fmt.Errorf("cannot convert pod: %w", err)
		}

		return &c, err
	case *batchv1.Job:
		c, err := kueueleuleu.ConvertJob(*tTyped)
		if err != nil {
			err = fmt.Errorf("cannot convert job: %w", err)
		}

		return &c, err
	case *batchv1.CronJob:
		c, err := kueueleuleu.ConvertCronJob(*tTyped)
		if err != nil {
			err = fmt.Errorf("cannot convert cronjob: %w", err)
		}

		return &c, err
	default:
		return nil, errUnsupportedConversion
	}
}
