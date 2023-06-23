package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	SVC_NAME = "producer"
	SVC_URL  = "http://slops-controller:62000/"
)

var (
	epIPs      []string
	key2EpMap  map[string][]string
	logger     *log.Logger
	curEpIndex = 0
)

func searchKeyInMap(targetKey string) string {
	for ip, keys := range key2EpMap {
		for _, key := range keys {
			if key == targetKey {
				return ip
			}
		}
	}
	return "nil"
}

func insertKeyInMap(key, targetIp string) {
	for ip := range key2EpMap {
		if ip == targetIp {
			key2EpMap[ip] = append(key2EpMap[ip], key)
			return
		}
	}
}

// Endpoints store information from the control plane
type Endpoints struct {
	Svcname string   `json:"Svcname"`
	Ips     []string `json:"Ips"`
}

func main() {
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)

	wg := sync.WaitGroup{}

	key2EpMap = map[string][]string{}
	epIPs = getEndpoints().Ips
	for _, ep := range epIPs {
		key2EpMap[ep] = []string{}
	}
	logger.Println("Endpoints established:", epIPs) // Debug

	// Go away and check for endpoints.
	epChannel := make(chan *Endpoints)
	wg.Add(1)
	go func(epChannel chan *Endpoints, wg *sync.WaitGroup) {
		defer wg.Done()
		// Get the IPs for the very first time.
		epChannel <- getEndpoints()

		// Check every 10 seconds if the endpoints have changed.
		ticker := time.NewTicker(time.Second * 10)
		for range ticker.C {
			epChannel <- getEndpoints()
		}

	}(epChannel, &wg)

	// Go away and keep updating the endpoints.
	wg.Add(1)
	go func() {
		defer wg.Done()
		endpoints := <-epChannel
		epIPs = endpoints.Ips
		// This would clean up the map and lead to bad behavior
		// but we ignore that since this part is not changing in our experiments.
		for _, ep := range endpoints.Ips {
			key2EpMap[ep] = []string{}
		}
	}()

	srv := http.Server{
		Addr:         ":9090",
		Handler:      routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	logger.Printf("Starting HTTP server on %s", srv.Addr)
	logger.Fatal(srv.ListenAndServe())

	// wg.Wait()
}

func routes() *gin.Engine {
	router := gin.Default()

	router.NoRoute(notFoundResponse)
	router.NoMethod(methodNotAllowedResponse)
	router.HandleMethodNotAllowed = true

	router.POST("/new", routeMsg)

	return router
}

func routeMsg(c *gin.Context) {
	var input struct {
		Key  string `json:"key"`
		Body string `json:"body"`
	}
	err := readJSON(c, &input)
	if err != nil {
		badRequestResponse(c, err)
		return
	}

	ip := searchKeyInMap(input.Key)
	if ip == "nil" {
		if curEpIndex >= len(epIPs) {
			curEpIndex = 0
		}
		logger.Printf("Number of servers %d, current index %d\n", len(epIPs), curEpIndex)
		ip = epIPs[curEpIndex]
		insertKeyInMap(input.Key, ip)
		curEpIndex++
	} else {
		logger.Println("Key found")
	}

	url := fmt.Sprintf("http://%s:2048/new", ip)

	json_data, err := json.MarshalIndent(input, "", "\t")

	res, err := http.Post(url, "application/json", bytes.NewBuffer(json_data))
	if err != nil {
		log.Printf("Error %v calling with %s\n", err, input.Key)
	} else {
		log.Printf("Response to %s: %v\n", input.Key, res)
	}
}

func getEndpoints() *Endpoints {
	resp, err := http.Get(SVC_URL + SVC_NAME)
	if err != nil {
		logger.Println("Error get request:", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Println("Error reading response:", err)
		return nil
	}

	var ep Endpoints
	err = json.Unmarshal(body, &ep)
	if err != nil {
		logger.Println("Error json unmarshalling:", err)
		return nil
	}
	logger.Println("Endpoints:", ep)
	return &ep
}

// The notFoundResponse() method will be used to send a 404 Not Found status code and
// JSON response to the client.
func notFoundResponse(c *gin.Context) {
	message := "the requested resource could not be found"
	errorResponse(c, http.StatusNotFound, message)
}

// The methodNotAllowedResponse() method will be used to send a 405 Method Not Allowed
// status code and JSON response to the client.
func methodNotAllowedResponse(c *gin.Context) {
	message := fmt.Sprintf("the %s method is not supported for this resource", c.Request.Method)
	errorResponse(c, http.StatusMethodNotAllowed, message)
}

func badRequestResponse(c *gin.Context, err error) {
	errorResponse(c, http.StatusBadRequest, err.Error())
}

// The errorResponse() method is a generic helper for sending JSON-formatted error
// messages to the client with a given status code. Note that we're using an interface{}
// type for the message parameter, rather than just a string type, as this gives us
// more flexibility over the values that we can include in the response.
func errorResponse(c *gin.Context, status int, msg any) {
	env := envelope{"error": msg}

	err := writeJSON(c.Writer, status, env, nil)
	if err != nil {
		logger.Println(err)
		c.Writer.WriteHeader(500)
	}
}

// Define an envelope type.
type envelope map[string]any

func writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)

	return nil
}

func readJSON(c *gin.Context, dst any) error {
	// Limit request body to 1 MB.
	maxBytes := 1_048_576
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxBytes))

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
			// In some circumstances Decode() may also return an io.ErrUnexpectedEOF error
			// for syntax errors in the JSON. So we check for this using errors.Is() and
			// return a generic error message. There is an open issue regarding this at
			// https://github.com/golang/go/issues/25956.

		case errors.Is(err, io.ErrUnexpectedEOF):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
			// These occur when the
			// JSON value is the wrong type for the target destination. If the error relates
			// to a specific field, then we include that in our error message to make it
			// easier for the client to debug.

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		// An io.EOF error will be returned by Decode() if the request body is empty. We
		// check for this with errors.Is() and return a plain-english error message
		// instead.
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		// If the JSON contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown
		// field "<name>"". We check for this, extract the field name from the error,
		// and interpolate it into our custom error message. Note that there's an open
		// issue at https://github.com/golang/go/issues/29035 regarding turning this
		// into a distinct error type in the future.
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		// A json.InvalidUnmarshalError error will be returned if we pass a non-nil
		// pointer to Decode(). We catch this and panic, rather than returning an error
		// to our handler. At the end of this chapter we'll talk about panicking
		// versus returning errors, and discuss why it's an appropriate thing to do in
		// this specific situation.
		// If the request body exceeds 1MB in size the decode will now fail with the
		// error "http: request body too large". There is an open issue about turning
		// this into a distinct error type at https://github.com/golang/go/issues/30715.
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}

	// Call Decode() again, using a pointer to an empty anonymous struct as the
	// destination. If the request body only contained a single JSON value this will
	// return an io.EOF error. So if we get anything else, we know that there is
	// additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}
