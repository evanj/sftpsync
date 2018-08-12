# sftpsync: Synchronize SFTP to Cloud Storage

Synchonizes an SFTP directory to a cloud storage bucket (S3 or Google Cloud Storage). This is mostly an experiment with Google's portable go-cloud library, but it might be useful to someone! If you need to work with FTP/SFTP and cloud storage, there are services you can use instead, such as [Conduit FTP, which provides an FTP/SFTP interface to cloud storage](https://www.conduitftp.com/).

## Usage

Assuming you have Go installed and your paths set up correctly:

1. `go get github.com/evanj/sftpsync`
2. `sftpsync sftp://example.com/directory s3://bucket/path`


## Authentication

### SSH/SFTP

`sftpsync` uses your SSH agent if you have it configured. If that fails, it will use your current user name and prompt for a password. Otherwise, you can specify a username and password in the URL as `sftp://user:pass@host/`.

### AWS S3

You must set the `AWS_REGION`, `AWS_ACCESS_KEY_ID`, and `AWS_SECRET_ACCESS_KEY` environment variables, as specified in Amazon's documentation.

### Google Cloud Storage

This uses Google Cloud's "[application default credentials](https://cloud.google.com/docs/authentication/production)". It will either use your `gcloud` account, the Compute Engine service account, or the key pointed to be the `GOOGLE_APPLICATION_CREDENTIALS` environment variable.
