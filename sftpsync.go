package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const defaultSSHPort = 22

type sftpSource struct {
	username string
	password string
	hostname string
	port     int
	path     string
}

func parseSource(input string) (sftpSource, error) {
	output := sftpSource{port: defaultSSHPort, path: "/"}
	sftpURL, err := url.Parse(input)
	if err != nil {
		return output, fmt.Errorf("invalid sftp URL: %s", err.Error())
	}

	if sftpURL.Scheme != "sftp" {
		return output, fmt.Errorf("scheme must be sftp (was %#v)", sftpURL.Scheme)
	}
	if sftpURL.Opaque != "" {
		return output, fmt.Errorf("invalid sftp URL")
	}
	if sftpURL.User != nil {
		output.username = sftpURL.User.Username()
		if output.username == "" {
			return output, fmt.Errorf("username cannot be empty")
		}
		isSet := false
		output.password, isSet = sftpURL.User.Password()
		if isSet && output.password == "" {
			return output, fmt.Errorf("password cannot be empty")
		}
	}

	output.hostname = sftpURL.Host
	parts := strings.Split(output.hostname, ":")
	if len(parts) == 2 {
		output.hostname = parts[0]
		output.port, err = strconv.Atoi(parts[1])
		if err != nil {
			return output, fmt.Errorf("invalid port: %s", err.Error())
		}
		if !(1 <= output.port && output.port < (1<<16)) {
			return output, fmt.Errorf("port out of range: %d", output.port)
		}
	}
	if output.hostname == "" {
		return output, fmt.Errorf("hostname cannot be empty")
	}

	if sftpURL.Path != "" {
		output.path = sftpURL.Path
	}

	if sftpURL.RawQuery != "" {
		return output, fmt.Errorf("query must be empty")
	}
	if sftpURL.Fragment != "" {
		return output, fmt.Errorf("fragment must be empty")
	}

	return output, nil
}

func main() {
	sourceFlag := flag.String("source", "", "source SFTP URL in the format sftp://username@hostname:port/dir")
	destinationFlag := flag.String("destination", "",
		"destination cloud storage bucket eg gs://bucket/dir or s3://bucket/dir")
	flag.Parse()

	source, err := parseSource(*sourceFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid -source %#v: %s\n", *sourceFlag, err.Error())
		os.Exit(1)
	}

	fmt.Println(source, *destinationFlag)
}
