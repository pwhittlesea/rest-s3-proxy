package main

import (
	// Input/Output
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"strconv"
	// Time
	"time"

	// Webserver
	"net/http"

	// AWS
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	// Loggers
	Info  *log.Logger
	Error *log.Logger

	// Health
	healthFile               string
	healthCheckCacheInterval int64
	lastHealthCheckTime      int64

	// Web server
	port string

	// AWS settings
	awsRegion, awsBucket string
	s3Session            *s3.S3
)

// Get an environment variable or use a default value if not set
func getEnvOrDefault(envName, defaultVal string, fatal bool) (envVal string) {
	envVal = os.Getenv(envName)
	if len(envVal) == 0 {
		if fatal {
			Error.Println("Unable to start as env " + envName + " is not defined")
			os.Exit(1)
		}
		envVal = defaultVal
		Info.Println("Using default " + envName + ": " + envVal)
	} else {
		Info.Println(envName + ": " + envVal)
	}
	return
}

// Get all the environment variables for this application
func getAllEnvVariables() {
	// Get the port that this webserver will be running upon
	port = getEnvOrDefault("PORT", "8000", false)

	// Get the AWS credentials
	awsRegion = getEnvOrDefault("AWS_REGION", "eu-west-1", false)
	awsBucket = getEnvOrDefault("AWS_BUCKET", "", true)
	getEnvOrDefault("AWS_ACCESS_KEY_ID", "", true)
	getEnvOrDefault("AWS_SECRET_ACCESS_KEY", "", true)

	// Get the path for the healthFile and the time to cache
	healthFile = getEnvOrDefault("HEALTH_FILE", ".rest-s3-proxy", false)

	// Get the time to wait between health checks (we dont want to hammer S3)
	healthIntervalString := getEnvOrDefault("HEALTH_CACHE_INTERVAL", "120", false)
	healthIntervalInt, err := strconv.ParseInt(healthIntervalString, 10, 64)
	if err != nil {
		panic(err)
	}
	healthCheckCacheInterval = healthIntervalInt
}

// Serve a request for a S3 file
func serveS3File(w http.ResponseWriter, r *http.Request) {
	var method = r.Method
	var path = r.URL.Path[1:] // Remove the / from the start of the URL

	// A file with no path cannot be served
	if path == "" {
		http.Error(w, "Path must be provided", http.StatusBadRequest)
		return
	}

	// Ensure the health endpoint is honoured
	if path == "healthz" {
		if method == "GET" {
			serveHealth(w, r)
		} else {
			http.Error(w, "/healthz is restricted to GET requests", http.StatusMethodNotAllowed)
		}
		return
	}

	Info.Println("Handling " + method + " request for '" + path + "'")

	switch method {
	case "GET":
		serveGetS3File(path, w, r)
	case "PUT":
		servePutS3File(path, w, r)
	case "DELETE":
		serveDeleteS3File(path, w, r)
	case "HEAD":
		serveHeadS3File(path, w, r)
	default:
		http.Error(w, "Method "+method+" not supported", http.StatusMethodNotAllowed)
	}
}

// Serve a HEAD request for a S3 file
func serveHeadS3File(filePath string, w http.ResponseWriter, r *http.Request) {
	input := &s3.HeadObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(filePath)}
	etag := r.Header.Get("ETag")
	if etag != "" {
		input.IfNoneMatch = &etag
	}
	resp, err := s3Session.HeadObject(input)
	if handleHTTPException(filePath, w, err) != nil {
		return
	}
	w.Header().Set("Content-Type", *resp.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", *resp.ContentLength))
	w.Header().Set("Last-Modified", resp.LastModified.String())
	w.Header().Set("Etag", *resp.ETag)
}

// Serve a health request
func serveHealth(w http.ResponseWriter, r *http.Request) {
	// Ensure that we can connect to the S3 bucket provided (every 10 seconds max)
	currentTime := time.Now().Unix()

	if (currentTime - lastHealthCheckTime) > healthCheckCacheInterval {
		Info.Println("Making health check for path '" + healthFile + "'")

		// Check that we have read permissions on the status file (we may not have listing permissions)
		params := &s3.GetObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(healthFile)}
		_, err := s3Session.GetObject(params)

		if handleHTTPException(healthFile, w, err) != nil {
			Error.Println("Health check failed")
			return
		}

		Info.Println("Health check passed")
		lastHealthCheckTime = currentTime
	}
	io.WriteString(w, "OK")
}

// Serve a GET request for a S3 file
func serveGetS3File(filePath string, w http.ResponseWriter, r *http.Request) {
	params := &s3.GetObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(filePath)}
	resp, err := s3Session.GetObject(params)

	w.Header().Set("Content-Type", *resp.ContentType)
	w.Header().Set("Last-Modified", resp.LastModified.String())
	w.Header().Set("Etag", *resp.ETag)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", *resp.ContentLength))

	if handleHTTPException(filePath, w, err) != nil {
		return
	}

	// File is ready to download
	io.Copy(w, resp.Body)
}

// Serve a PUT request for a S3 file
func servePutS3File(filePath string, w http.ResponseWriter, r *http.Request) {
	// Convert the uploaded body to a byte array TODO fix this for large sizes
	b, err := ioutil.ReadAll(r.Body)

	if handleHTTPException(filePath, w, err) != nil {
		return
	}

	params := &s3.PutObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(filePath), Body: bytes.NewReader(b)}

	resp, err := s3Session.PutObject(params)

	if handleHTTPException(filePath, w, err) != nil {
		return
	}
	w.Header().Set("ETag", *resp.ETag)

	// File has been created TODO do not return a http.StatusCreated if the file was updated
	http.Redirect(w, r, "/"+filePath, http.StatusCreated)
}

// Serve a DELETE request for a S3 file
func serveDeleteS3File(filePath string, w http.ResponseWriter, r *http.Request) {
	params := &s3.DeleteObjectInput{Bucket: aws.String(awsBucket), Key: aws.String(filePath)}
	_, err := s3Session.DeleteObject(params)

	if handleHTTPException(filePath, w, err) != nil {
		return
	}

	// File has been deleted
	w.WriteHeader(http.StatusNoContent)
}

// Handle an exception and write to response
func handleHTTPException(path string, w http.ResponseWriter, err error) (e error) {
	if err != nil {
		if awsError, ok := err.(awserr.Error); ok {
			// aws error
			switch awsError.Code() {
			case "MissingContentLength":
				http.Error(w, "Bad Request", http.StatusBadRequest)
			case "NotModified":
				http.Error(w, "Object not modified", http.StatusNotModified)
			case "NoSuchKey", "NotFound":
				http.Error(w, "Path '"+path+"' not found: "+awsError.Message(), http.StatusNotFound)
			default:
				origErr := awsError.OrigErr()
				cause := ""
				if origErr != nil {
					cause = " (Cause: " + origErr.Error() + ")"
				}
				http.Error(w, "An internal error occurred: "+awsError.Code()+" = "+awsError.Message()+cause, http.StatusInternalServerError)
			}
		} else {
			// golang error
			http.Error(w, "An internal error occurred: "+err.Error(), http.StatusInternalServerError)
		}
	}
	return err
}

// Initialise loggers
func initLogging(infoHandle io.Writer, errorHandle io.Writer) {
	Info = log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

// Main method
func main() {
	initLogging(os.Stdout, os.Stderr)

	// Reset health check status
	lastHealthCheckTime = 0

	// Set up all the environment variables
	getAllEnvVariables()

	// Set up the S3 connection
	s3Session = s3.New(session.New(), &aws.Config{Region: aws.String(awsRegion)})

	Info.Println("Startup complete")

	// Run the webserver
	http.HandleFunc("/", serveS3File)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		Error.Println("ListenAndServe: ", err)
		os.Exit(1)
	}
}
