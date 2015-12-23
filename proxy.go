package main

import (
	// Input/Output
	"io"
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
	port                   string
	aws_region, aws_bucket string
	s3_session             *s3.S3
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
	aws_region = getEnvOrDefault("AWS_REGION", "eu-west-1", false)
	aws_bucket = getEnvOrDefault("AWS_BUCKET", "", true)
	getEnvOrDefault("AWS_ACCESS_KEY_ID", "", true)
	getEnvOrDefault("AWS_SECRET_ACCESS_KEY", "", true)
}

// Serve a request for a S3 file
func serveS3File(w http.ResponseWriter, r *http.Request) {
	var method = r.Method
	var path = r.URL.Path[1:] // Remove the / from the start of the URL
	switch method {
	case "GET":
		serveGetS3File(path, w, r)
	default:
		http.Error(w, "Method "+method+" not supported", 405)
	}
}

// Serve a GET request for a S3 file
func serveGetS3File(file_path string, w http.ResponseWriter, r *http.Request) {
	params := &s3.GetObjectInput{Bucket: aws.String(aws_bucket), Key: aws.String(file_path)}
	resp, err := s3_session.GetObject(params)
	if err != nil {
		if awserr, ok := err.(awserr.Error); ok {
			switch awserr.Code() {
			case "NoSuchKey":
				http.Error(w, "Requested file not found", 404)
			default:
				http.Error(w, "An internal error occurred: "+awserr.Code()+" = "+awserr.Message(), 500)
			}
		} else {
			http.Error(w, "An internal error occurred: "+awserr.Message(), 500)
		}
	} else {
		io.Copy(w, resp.Body)
	}
}

// Main method
func main() {
	// Set up all the environment variables
	getAllEnvVariables()

	s3_session = s3.New(session.New(), &aws.Config{Region: aws.String(aws_region)})

	// Run the webserver
	http.HandleFunc("/", serveS3File)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
