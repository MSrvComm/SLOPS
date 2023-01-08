package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

func Generator(next chan bool, abort chan struct{}, num_keys uint64) <-chan string {
	ch := make(chan string)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	zipf := rand.NewZipf(r, 3.14, 2.72, num_keys)
	go func() {
		defer close(ch)
		for {
			select {
			case <-next:
				ch <- fmt.Sprintf("location_%d", zipf.Uint64())
			case <-abort:
				return
			}
		}
	}()
	return ch
}

func main() {
	// User args.
	var rt int
	var keys int
	var iters int
	var url string

	// flags declaration.
	flag.IntVar(&rt, "rate", 200, "Rate of requests per second")
	flag.IntVar(&keys, "keys", 2800, "Number of keys to choose from")
	flag.IntVar(&iters, "iter", 1_000, "Number of times the test is run")
	flag.StringVar(&url, "url", "https://httpbin.org/anything/", "URL to test")

	flag.Parse()

	limiter := rate.NewLimiter(rate.Limit(rt), rt)
	ctx := context.Background()

	c := make(chan os.Signal, 1)
	// Passing no signals to Notify means that
	// all signals will be sent to the channel.
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// num_keys := uint64(28_000_000)
	num_keys := uint64(keys)
	abort := make(chan struct{})
	next := make(chan bool)
	ch := Generator(next, abort, num_keys)

	// n := 1_000_000
	// n := 1_000
	i := 0

	frequencies := make(map[string]int)

	go func() {
		<-c
		log.Println("Rate:", rt)
		log.Println("Iters:", iters)
		log.Println("length freqs:", len(frequencies))
		log.Println("Total requests made:", i)
		os.Exit(1)
	}()

	for i < iters {
		if err := limiter.Wait(ctx); err != nil {
			log.Println("Limiter:", err)
		}
		next <- true
		x := <-ch

		var data struct {
			Key  string `json:"key"`
			Body string `json:"body"`
		}

		data.Key = x
		data.Body = fmt.Sprintf("%s message body", x)

		json_data, err := json.MarshalIndent(data, "", "\t")

		if err != nil {
			log.Fatal(err)
		}

		log.Println("URL:", url)
		res, err := http.Post(url, "application/json", bytes.NewBuffer(json_data))
		if err != nil {
			log.Printf("Error %v calling with %s\n", err, x)
		} else {
			log.Printf("Response to %s: %v\n", x, res)
		}

		if v, exists := frequencies[x]; exists {
			frequencies[x] = v + 1
		} else {
			frequencies[x] = 1
		}
		i += 1
	}
	close(abort)
	log.Println(frequencies)
	high_freqs := make(map[string]int)
	for k, v := range frequencies {
		if v > 100 {
			high_freqs[k] = v
		}
	}
	output(high_freqs, frequencies, i)
}

func output(high_freqs, frequencies map[string]int, i int) {
	log.Println(high_freqs)
	log.Println("length freqs:", len(frequencies))
	log.Println("length high freqs:", len(high_freqs))
	log.Println("Total requests made:", i)
}
