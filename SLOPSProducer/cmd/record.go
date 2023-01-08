package main

// Record is used in the lossy count alogrithm.
type Record struct {
	Key    string // Key being tracked.
	Count  int    // Number of messages for this key.
	Bucket int    // Current lossy count bucket.
}
