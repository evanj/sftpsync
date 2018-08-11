package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSource(t *testing.T) {
	invalid := []struct {
		input         string
		expectedError string
	}{
		{"http://host", "scheme must be sftp"},
		{"sftp:host", "invalid sftp URL"},
		{"sftp://", "hostname cannot be empty"},
		{"sftp://@host", "username cannot be empty"},
		{"sftp://u:@host", "password cannot be empty"},
		{"sftp://host?query", "query must be empty"},
		{"sftp://host/?query", "query must be empty"},
		{"sftp://host/#fragment", "fragment must be empty"},
		{"sftp://user@", "hostname cannot be empty"},
		{"sftp://user@:42", "hostname cannot be empty"},
		{"sftp://user@host:", "invalid port"},
		{"sftp://user@host:-42/", "port out of range"},
		{"sftp://user@host:/", "invalid port"},
		{"sftp://user@host:0/", "port out of range"},
		{"sftp://user@host:65536/", "port out of range"},
	}
	for i, test := range invalid {
		out, err := parseSource(test.input)
		if err == nil {
			t.Errorf("%d: parseSource(%#v) = %#v, %#v; expected error %#v",
				i, test.input, out, err, test.expectedError)
		} else if !strings.Contains(err.Error(), test.expectedError) {
			t.Errorf("%d: parseSource(%#v) = %#v, %#v; expected error to contain %#v",
				i, test.input, out, err, test.expectedError)
		}
	}

	valid := []struct {
		input  string
		output sftpSource
	}{
		{"sftp://host", sftpSource{"", "", "host", 22, "/"}},
		{"sftp://host:1", sftpSource{"", "", "host", 1, "/"}},
		{"sftp://host:65535", sftpSource{"", "", "host", 65535, "/"}},
		{"sftp://host:1/", sftpSource{"", "", "host", 1, "/"}},
		{"sftp://host:1/", sftpSource{"", "", "host", 1, "/"}},
		{"sftp://host:1/path", sftpSource{"", "", "host", 1, "/path"}},
		{"sftp://user:pass@host:12345/path", sftpSource{"user", "pass", "host", 12345, "/path"}},
	}
	for i, test := range valid {
		out, err := parseSource(test.input)
		if err != nil {
			t.Errorf("%d: parseSource(%#v) = %#v, %#v; expected output %#v",
				i, test.input, out, err, test.output)
		} else if !reflect.DeepEqual(out, test.output) {
			t.Errorf("%d: parseSource(%#v) = %#v, %#v; expected output %#v",
				i, test.input, out, err, test.output)
		}
	}
}
