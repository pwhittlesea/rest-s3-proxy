package main

import (
	// Input/Output
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"

	// Webserver
	"net/http"

	// AWS
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	port                 string
	awsRegion, awsBucket string
	s3Session            *s3.S3
)

// Get an environment variable or use a default value if not set
func getEnvOrDefault(envName, defaultVal string, fatal bool) (envVal string) {
	envVal = os.Getenv(envName)
	if len(envVal) == 0 {
		if fatal {
			log.Fatal("Unable to start as env " + envName + " is not defined")
		}
		envVal = defaultVal
		log.Output(1, "Using default "+envName+": "+envVal)
	} else {
		log.Output(1, envName+": "+envVal)
	}
	return
}

// Get all the environment varables for this application
func getAllEnvVariables() {
	// Get the port that this webserver will be running upon
	port = getEnvOrDefault("PORT", "8000", false)

	// Get the AWS credentials
	awsRegion = getEnvOrDefault("AWS_REGION", "eu-west-1", false)
	awsBucket = getEnvOrDefault("AWS_BUCKET", "", true)
	getEnvOrDefault("AWS_ACCESS_KEY_ID", "", true)
	getEnvOrDefault("AWS_SECRET_ACCESS_KEY", "", true)
}

// Serve a request for a S3 file
func serveS3File(w http.ResponseWriter, r *http.Request) {
	var method = r.Method
	var path = r.URL.Path[1:] // Remove the / from the start of the URL

	// A file with no path cannot be served
	if path == "" {
		http.Error(w, "Path must be provided", 400)
		return
	}

	switch method {
	case "GET":
		serveGetS3File(path, w, r)
	case "PUT":
		servePutS3File(path, w, r)
	case "DELETE":
		serveDeleteS3File(path, w, r)
	default:
		http.Error(w, "Method "+method+" not supported", 405)
	}
}

// Serve a GET request for a S3 file
func serveGetS3File(filePath string, w http.ResponseWriter, r *http.Request) {
	params := &s3.GetObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(filePath)}
	resp, err := s3Session.GetObject(params)

	if handleHTTPException(w, err) != nil {
		return
	}

	// File is ready to download
	io.Copy(w, resp.Body)
}

// Serve a PUT request for a S3 file
func servePutS3File(filePath string, w http.ResponseWriter, r *http.Request) {
	// Convert the uploaded body to a byte array TODO fix this for large sizes
	b, err := ioutil.ReadAll(r.Body)

	if handleHTTPException(w, err) != nil {
		return
	}

	params := &s3.PutObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(filePath), Body: bytes.NewReader(b)}
	_, err = s3Session.PutObject(params)

	if handleHTTPException(w, err) != nil {
		return
	}

	// File has been created TODO do not return a 201 if the file was updated
	http.Redirect(w, r, "/"+filePath, 201)
}

// Serve a DELETE request for a S3 file
func serveDeleteS3File(filePath string, w http.ResponseWriter, r *http.Request) {
	params := &s3.DeleteObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(filePath)}
	_, err := s3Session.DeleteObject(params)

	if handleHTTPException(w, err) != nil {
		return
	}

	// File has been deleted
	http.Redirect(w, r, "/", 200)
}

// Handle an exception and write to response
func handleHTTPException(w http.ResponseWriter, err error) (e error) {
	if err != nil {
		if awserr, ok := err.(awserr.Error); ok {
			// aws error
			switch awserr.Code() {
			case "NoSuchKey":
				http.Error(w, "Not found: "+awserr.Message(), 404)
			default:
				http.Error(w, "An internal error occurred: "+awserr.Code()+" = "+awserr.Message(), 500)
			}
		} else {
			// golang error
			http.Error(w, "An internal error occurred: "+err.Error(), 500)
		}
	}
	return err
}

// Main method
func main() {
	// Set up all the environment variables
	getAllEnvVariables()

	s3Session = s3.New(session.New(), &aws.Config{Region: aws.String(awsRegion)})

	// Run the webserver
	http.HandleFunc("/", serveS3File)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
