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

func TestParseBucket(t *testing.T) {
	invalid := []struct {
		input         string
		expectedError string
	}{
		{"http://bucket", "invalid scheme"},
		{"gs:host", "invalid URL"},
		{"gs://", "bucket cannot be empty"},
		{"gs:///path", "bucket cannot be empty"},
		{"gs://user@bucket/", "username/password cannot be provided"},
		{"gs://:password@bucket/", "username/password cannot be provided"},
		{"gs://bucket:42/", "bucket cannot contain :"},
		{"gs://bucket?query", "query must be empty"},
		{"gs://bucket/#fragment", "fragment must be empty"},
	}
	for i, test := range invalid {
		out, err := parseCloudStorageURL(test.input)
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
		output cloudStorageURL
	}{
		{"gs://bucket", cloudStorageURL{"gs", "bucket", "/"}},
		{"gs://bucket/", cloudStorageURL{"gs", "bucket", "/"}},
		{"gs://bucket/path", cloudStorageURL{"gs", "bucket", "/path"}},
		{"s3://bucket/path/", cloudStorageURL{"s3", "bucket", "/path/"}},
	}
	for i, test := range valid {
		out, err := parseCloudStorageURL(test.input)
		if err != nil {
			t.Errorf("%d: parseSource(%#v) = %#v, %#v; expected output %#v",
				i, test.input, out, err, test.output)
		} else if !reflect.DeepEqual(out, test.output) {
			t.Errorf("%d: parseSource(%#v) = %#v, %#v; expected output %#v",
				i, test.input, out, err, test.output)
		}
	}
}

func TestDestinationPath(t *testing.T) {
	tests := []struct {
		srcRoot, srcPath, dstRoot, expected string
	}{
		{"/", "/file", "/dest", "dest/file"},
		{"/", "/file", "/dest/", "dest/file"},
		{"/a/b", "/a/b/c/d/file", "/dest/", "dest/c/d/file"},
	}
	for i, test := range tests {
		out := makeDestinationPath(test.srcRoot, test.srcPath, test.dstRoot)
		if out != test.expected {
			t.Errorf("%d: makeDestinationPath(%#v, %#v, %#v) = %#v; expected %#v",
				i, test.srcRoot, test.srcPath, test.dstRoot, out, test.expected)
		}
	}
}
