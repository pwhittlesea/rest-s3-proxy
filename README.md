# REST S3 Proxy [![Build Status](https://travis-ci.org/pwhittlesea/rest-s3-proxy.svg?branch=develop)](https://travis-ci.org/pwhittlesea/rest-s3-proxy)
Microservice to provide S3-backed persistent storage to a cluster using REST

## Example
Given a S3 bucket named ***my-bucket*** with a file in it called ***my-file.xml***.

Starting the proxy with the bucket as the *AWS_BUCKET* argument you should be able to make a HTTP GET request to http://localhost:8000/my-file.xml and get the contents of ***my-file.xml*** back.

## Building
Run the following to checkout dependencies:
```
go get -u github.com/aws/aws-sdk-go/...
```

## Running
The application requires several environment variables in order to run.
Below is an example execution:

```
PORT=9999 \
AWS_REGION=the-region \
AWS_ACCESS_KEY_ID=<redacted> \
AWS_SECRET_ACCESS_KEY=<redacted> \
AWS_BUCKET=the-bucket-name \
./rest-s3-proxy
```

### Arguments
#### PORT
The port number the application will listen on.

*Optional - Default: 8000*

#### AWS_REGION
The AWS region the bucket resides in.

*Optional - Default: eu-west-1*

#### AWS_ACCESS_KEY_ID
The access key ID for AWS authentication.
**Must** have S3 read (and write if HTTP PUT is to be used) permissions.

*Mandatory - Application will exit if not present*

#### AWS_SECRET_ACCESS_KEY
The secret AWS access key for authentication.

*Mandatory - Application will exit if not present*

#### AWS_BUCKET
The name of the bucket.

*Mandatory - Application will exit if not present*
