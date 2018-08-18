package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/go-cloud/blob"
	"github.com/google/go-cloud/blob/gcsblob"
	"github.com/google/go-cloud/blob/s3blob"
	"github.com/google/go-cloud/gcp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/terminal"
)

const defaultSSHPort = 22

const schemeSFTP = "sftp"
const schemeGCS = "gs"
const schemeS3 = "s3"

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

	if sftpURL.Scheme != schemeSFTP {
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

type cloudStorageURL struct {
	provider string
	bucket   string
	path     string
}

func parseCloudStorageURL(input string) (cloudStorageURL, error) {
	output := cloudStorageURL{path: "/"}
	storageURL, err := url.Parse(input)
	if err != nil {
		return output, fmt.Errorf("invalid URL: %s", err.Error())
	}

	if !(storageURL.Scheme == schemeGCS || storageURL.Scheme == schemeS3) {
		return output, fmt.Errorf("invalid scheme: %s", storageURL.Scheme)
	}
	output.provider = storageURL.Scheme

	if storageURL.Opaque != "" {
		return output, fmt.Errorf("invalid URL")
	}

	if storageURL.User != nil {
		return output, fmt.Errorf("username/password cannot be provided for cloud storage")
	}

	output.bucket = storageURL.Host
	if strings.ContainsRune(output.bucket, ':') {
		return output, fmt.Errorf("bucket cannot contain :")
	}
	if output.bucket == "" {
		return output, fmt.Errorf("bucket cannot be empty")
	}

	if storageURL.Path != "" {
		output.path = storageURL.Path
	}

	if storageURL.RawQuery != "" {
		return output, fmt.Errorf("query must be empty")
	}
	if storageURL.Fragment != "" {
		return output, fmt.Errorf("fragment must be empty")
	}

	return output, nil
}

func defaultClientConfig() *ssh.ClientConfig {
	config := &ssh.ClientConfig{HostKeyCallback: ssh.InsecureIgnoreHostKey()}

	// attempt to use ssh agent if configured
	if aConn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		auth := ssh.PublicKeysCallback(agent.NewClient(aConn).Signers)
		config.Auth = append(config.Auth, auth)
	}

	currentUser, err := user.Current()
	if err != nil {
		// the lookup failed: we can't attempt any defaults
		return config
	}
	config.User = currentUser.Username

	// TODO: Read OpenSSH's config files to find private keys etc
	return config
}

func makePasswordPromptFunc(username string, host string) func() (string, error) {
	return func() (string, error) {
		os.Stdout.WriteString(fmt.Sprintf("%s@%s's Password: ", username, host))
		passwordBytes, err := terminal.ReadPassword(0)
		os.Stdout.Write([]byte("\n"))
		return string(passwordBytes), err
	}
}

// Returns both the SSH connection and SFTP client since they both need to be closed
func connectSFTP(serverConfig sftpSource) (ssh.Conn, *sftp.Client, error) {
	clientConfig := defaultClientConfig()

	if serverConfig.username != "" {
		clientConfig.User = serverConfig.username
	}
	if serverConfig.password != "" {
		clientConfig.Auth = append(clientConfig.Auth, ssh.Password(serverConfig.password))
	} else {
		promptFunc := makePasswordPromptFunc(clientConfig.User, serverConfig.hostname)
		clientConfig.Auth = append(clientConfig.Auth, ssh.PasswordCallback(promptFunc))
	}
	log.Printf("WTF %d auth", len(clientConfig.Auth))

	addr := fmt.Sprintf("%s:%d", serverConfig.hostname, serverConfig.port)
	sshClient, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return nil, nil, err
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, nil, err
	}
	return sshClient, sftpClient, err
}

type logRoundTripper struct {
	orig http.RoundTripper
}

func (l *logRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	log.Printf("req: %s %s", req.Method, req.URL.String())
	resp, origErr := l.orig.RoundTrip(req)
	log.Printf("resp: %d %#v", resp.StatusCode, resp.Header)
	buf := &bytes.Buffer{}
	_, err := io.Copy(buf, resp.Body)
	err2 := resp.Body.Close()
	if err != nil {
		return resp, err
	}
	if err2 != nil {
		return resp, err2
	}
	log.Printf("body: %s", string(buf.Bytes()))
	resp.Body = ioutil.NopCloser(buf)
	return resp, origErr
}

func openBucket(bucketURL cloudStorageURL) (*blob.Bucket, error) {
	ctx := context.Background()
	if bucketURL.provider == schemeGCS {
		credentials, err := gcp.DefaultCredentials(ctx)
		if err != nil {
			return nil, err
		}
		client, err := gcp.NewHTTPClient(gcp.DefaultTransport(), gcp.CredentialsTokenSource(credentials))
		if err != nil {
			return nil, err
		}
		return gcsblob.OpenBucket(ctx, bucketURL.bucket, client)
	} else if bucketURL.provider == schemeS3 {
		region := os.Getenv("AWS_REGION")
		if region == "" {
			return nil, fmt.Errorf("Must specify AWS_REGION environment variable")
		}
		config := &aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewEnvCredentials(),
		}
		sess, err := session.NewSession(config)
		if err != nil {
			return nil, err
		}
		return s3blob.OpenBucket(ctx, sess, bucketURL.bucket)
	}

	return nil, fmt.Errorf("unsupported provider: %s", bucketURL.provider)
}

func makeDestinationPath(srcRoot string, srcPath string, dstRoot string) string {
	if srcRoot == "" {
		panic("invalid srcRoot " + srcRoot)
	}
	if srcRoot[len(srcRoot)-1] != '/' {
		srcRoot += "/"
	}
	if dstRoot == "" {
		panic("invalid dstRoot " + srcRoot)
	}

	if !strings.HasPrefix(srcPath, srcRoot) {
		panic(fmt.Sprintf("srcPath %#v must start with srcRoot %#v", srcPath, srcRoot))
	}

	relative := srcPath[len(srcRoot):]
	out := path.Join(dstRoot, relative)
	if out[0] != '/' {
		panic(fmt.Sprintf("invalid output: %#v", out))
	}
	return out[1:]
}

func copySFTPToBucket(
	sftpClient *sftp.Client, sftpPath string, bucket *blob.Bucket, bucketPath string,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, err := sftpClient.Open(sftpPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	writer, err := bucket.NewWriter(ctx, bucketPath, nil)
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = io.Copy(writer, reader)
	if err != nil {
		// cancel the upload so it fails and does not create output; GCP storage writer will do this
		cancel()
		return err
	}
	err = reader.Close()
	if err != nil {
		cancel()
		return err
	}
	return writer.Close()
}

func sync(sftpClient *sftp.Client, srcPath string, bucket *blob.Bucket, dstPath string) error {
	ctx := context.Background()
	walker := sftpClient.Walk(srcPath)
	for walker.Step() {
		if walker.Err() != nil {
			return walker.Err()
		}
		if walker.Stat().IsDir() {
			continue
		}

		// check if this file exists in the bucket
		// TODO: Should list files instead, but that doesn't exist (yet?):
		// https://github.com/google/go-cloud/issues/241
		bucketPath := makeDestinationPath(srcPath, walker.Path(), dstPath)
		reader, err := bucket.NewRangeReader(ctx, bucketPath, 0, 0)
		needsUpload := true
		if err != nil {
			// ignore not exists error: means we need to do the upload
			if !blob.IsNotExist(err) {
				return err
			}
		} else {
			// the file exists: check if it is already up to date
			// truncate times to the nearest second: some SFTP implementation don't support
			// seconds: https://tools.ietf.org/html/draft-ietf-secsh-filexfer-13#section-7.7
			sftpTime := walker.Stat().ModTime().Truncate(time.Second)
			bucketTime := reader.ModTime().Truncate(time.Second)
			if bucketTime.IsZero() {
				// Workaround go-cloud bug: ignore mtimes for Google Cloud
				bucketTime = sftpTime.Add(time.Second)
			}
			// if the bucket time is older than the SFTP time: we assume we need an update
			// we can't control the modification times on the buckets, so we will assume time
			// moves forward in some sane way
			if bucketTime.After(sftpTime) && walker.Stat().Size() == reader.Size() {
				log.Printf("%s: skipping; mtime and size match", walker.Path())
				needsUpload = false
			}
			err = reader.Close()
			if err != nil {
				return err
			}
		}

		if needsUpload {
			log.Printf("%s: copying ...", walker.Path())
			err = copySFTPToBucket(sftpClient, walker.Path(), bucket, bucketPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	// sourceFlag := flag.String("source", "", "source SFTP URL in the format sftp://username@hostname:port/dir")
	// destinationFlag := flag.String("destination", "",
	// 	"destination cloud storage bucket eg gs://bucket/dir or s3://bucket/dir")
	// flag.Parse()
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: sftpsync (source SFTP URL) (destination cloud storage URL)\n")
		fmt.Fprintf(os.Stderr, "    source: Format sftp://username:password@hostname:port/dir\n")
		fmt.Fprintf(os.Stderr, "    destination: Format gs://bucket/dir or s3://bucket/dir\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "sftpsync will transfer files relative to the source directory. It checks\n")
		fmt.Fprintf(os.Stderr, "the file size and modification times to test if it needs to copy\n")
		os.Exit(1)
	}
	sourceString := os.Args[1]
	destinationString := os.Args[2]

	source, err := parseSource(sourceString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid source %#v: %s\n", sourceString, err.Error())
		os.Exit(1)
	}

	destination, err := parseCloudStorageURL(destinationString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid destination %#v: %s\n", destinationString, err.Error())
		os.Exit(1)
	}

	sshClient, sftpClient, err := connectSFTP(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to %s: %s\n", sourceString, err.Error())
		os.Exit(1)
	}
	defer sftpClient.Close()
	defer sshClient.Close()

	bucket, err := openBucket(destination)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open bucket %#v: %s\n", destinationString, err.Error())
		os.Exit(1)
	}

	err = sync(sftpClient, source.path, bucket, destination.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync: %s\n", err.Error())
		os.Exit(1)
	}
	err = sftpClient.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed closing SFTP: %s\n", err.Error())
		os.Exit(1)
	}
	err = sshClient.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed closing SSH: %s\n", err.Error())
		os.Exit(1)
	}
}
