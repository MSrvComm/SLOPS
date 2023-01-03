# SLOPS Message Generator

This generates messages using the Zipf distribution.

## Parameters

```
./SLOPSClient -h
Usage of ./SLOPSClient:
  -burst int
        Maximum burst allowed (default 200)
  -iter int
        Number of times the test is run (default 1000)
  -keys int
        Number of keys to choose from (default 2800)
  -rate float
        Rate of requests per second (default 200)
  -url string
        URL to test (default "https://httpbin.org/anything/")
```

## Usage

Get the exposed port for the SLOPS service:

```bash
PORT=$(kubectl get svc producer -o go-template='{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}')
```

```bash
go run main.go -url http://localhost:$PORT/new
```