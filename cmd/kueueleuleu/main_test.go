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
	"bytes"
	_ "embed"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/cronjob_output.yaml
	cronjobExpectedOutput string
	//go:embed testdata/job_output.yaml
	jobExpectedOutput string
	//go:embed testdata/pod_and_job_output.yaml
	podAndJobExpectedOutput string
	//go:embed testdata/pod_output.yaml
	podExpectedOutput string
)

func Test_convertYAMLToStdout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inputFilename string
		expected      string
	}{
		{
			inputFilename: "testdata/cronjob_input.yaml",
			expected:      cronjobExpectedOutput,
		},
		{
			inputFilename: "testdata/job_input.yaml",
			expected:      jobExpectedOutput,
		},
		{
			inputFilename: "testdata/pod_and_job_input.yaml",
			expected:      podAndJobExpectedOutput,
		},
		{
			inputFilename: "testdata/pod_input.yaml",
			expected:      podExpectedOutput,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.inputFilename, func(t *testing.T) {
			t.Parallel()

			buffer := &bytes.Buffer{}

			convertYAML(testCase.inputFilename, buffer)

			got, err := io.ReadAll(buffer)
			require.NoError(t, err)

			assert.Equal(t, testCase.expected, string(got))
		})
	}
}
