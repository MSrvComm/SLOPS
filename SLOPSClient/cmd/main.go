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

// partitionKeyMap combines sets of keys that will get mapped to sets of partitions.
// For example, keys [3, 18, 25, 34] will be mapped by the hash function to partition 0.
// Keys [11, 26, 28, 33, 39] to partition 1 and [10, 27, 29, 32, 38] to partition 2.
// PartitionSet 0 in this map combines keys that will get mapped to partitions 0, 1 and 2 into a single set.
// Similarly, partitionSet 1 combines keys for partitions 4, 5 and 6 while partitionSet 2 combines
// keys that will be mapped to partitions 7 through 9.
var partitionKeyMap = map[int][]int{
	0: {3, 18, 25, 34, 11, 26, 28, 33, 39, 10, 27, 29, 32, 38},
	1: {4, 13, 20, 31, 5, 12, 21, 30, 6, 8, 15, 22},
	2: {7, 9, 14, 23, 0, 17, 37, 1, 16, 36, 2, 19, 24, 35},
}

// TargetedGenerator sends generates keys that will be mapped to any one set of partitions at any point of time.
// This generator is designed to overwhelm Kafka by targeting load to a smaller set of partitions
// than available on the system.
func TargetedGenerator(next chan bool, abort chan struct{}) <-chan string {
	ch := make(chan string)
	go func() {
		count := 0
		set := 0
		for {
			select {
			case <-next:
				// If we have already sent 3K messages.
				// Change the partition set.
				if count%3000 == 0 {
					set++
					if set > 2 {
						set = 0
					}
				}
				// Increment messaages sent.
				count++
				// Pick a random key within the partition set.
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				ind := r.Intn(len(partitionKeyMap[set]))
				ch <- fmt.Sprintf("location_%d", partitionKeyMap[set][ind])
			case <-abort:
				return
			}
		}
	}()
	return ch
}

func ZipfGenerator(next chan bool, abort chan struct{}, num_keys uint64) <-chan string {
	ch := make(chan string)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// zipf := rand.NewZipf(r, 3.14, 2.72, num_keys)
	zipf := rand.NewZipf(r, 1.1, 2.72, num_keys)
	// zipf := rand.NewZipf(r, 5, 2.72, num_keys)
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
	flag.StringVar(&url, "url", "dummy", "URL to test")

	flag.Parse()

	limiter := rate.NewLimiter(rate.Limit(rt), rt)
	ctx := context.Background()

	c := make(chan os.Signal, 1)
	// Passing no signals to Notify means that
	// all signals will be sent to the channel.
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	num_keys := uint64(keys)
	abort := make(chan struct{})
	next := make(chan bool)
	ch := ZipfGenerator(next, abort, num_keys)
	// ch := TargetedGenerator(next, abort)

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

		log.Printf("URL: %s, data: %v\n", url, json_data)
		if url != "dummy" { // dummy means user wants to test.
			res, err := http.Post(url, "application/json", bytes.NewBuffer(json_data))
			if err != nil {
				log.Printf("Error %v calling with %s\n", err, x)
			} else {
				log.Printf("Response to %s: %v\n", x, res)
			}
		}

		if v, exists := frequencies[x]; exists {
			frequencies[x] = v + 1
		} else {
			frequencies[x] = 1
		}
		i += 1
	}
	close(abort)
	output(frequencies, i, keys)
}

func output(frequencies map[string]int, i, keys int) {
	log.Println(frequencies)
	high_freqs := make(map[string]int)
	high_freq_reqs := 0
	for k, v := range frequencies {
		if v > 100 {
			high_freqs[k] = v
			high_freq_reqs += v
		}
	}
	log.Println(high_freqs)
	log.Println("length freqs:", len(frequencies))
	log.Println("length high freqs:", len(high_freqs))
	log.Println("#High Frequency Requests:", high_freq_reqs)
	log.Println("Total requests made:", i)
	log.Println("Keyspace size:", keys)
}
