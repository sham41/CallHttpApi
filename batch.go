package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type BatchInfo struct {
	Index int
	Total int
	Size  int
}

type BatchJob struct {
	Info BatchInfo
	Data []interface{}
}

// processBatch processes the input data in batches and sends POST requests to the specified URL.
func (a *Api) processBatch(data []byte, url string) error {

	itemsKey := a.itemsKey

	var payload map[string]interface{}

	// unmarshal the original payload to get the items array
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	// get the items array from the payload
	rawItems, ok := payload[itemsKey]
	if !ok {
		return fmt.Errorf("#Error: key '%s' not found", itemsKey)
	}

	// assert that the items is an array
	items, ok := rawItems.([]interface{})
	if !ok {
		return fmt.Errorf("#Error: key '%s' is not array", itemsKey)
	}

	// if items count is less than batch size, process in one request
	if len(items) <= a.batchSize {

		// send POST request with the original payload
		resp, err := a.doRequest("POST", url, data)
		if err != nil {
			return err
		}

		// log the response for the the original payload
		a.writeResponseLog("POST", url, data, resp)

		// no batching needed
		return nil
	}

	// split items into batches
	batches := split(items, a.batchSize)

	// total number of batches
	totalbatches := len(batches)

	// channel to collect errors from workers
	errChan := make(chan error, totalbatches)

	// create a channel for batches
	jobs := make(chan BatchJob, totalbatches)

	// wait group to wait for all workers to finish
	var wg sync.WaitGroup

	// mutex for logging to prevent interleaving logs from different goroutines
	var mu sync.Mutex

	// record the start time of the batch operation
	start := time.Now()

	// =========================
	// START logging ( with mutex )
	mu.Lock()
	a.writeOperationStart("POST", url, totalbatches)
	mu.Unlock()

	// start worker goroutines
	for i := 0; i < a.batchWorkers; i++ {

		// start a worker goroutine
		go func() {

			// process batches from the channel until it's closed
			for job := range jobs {

				// process the batch and send error to channel if it occurs
				err := processOneBatch(a, payload, job, itemsKey, url, &mu)

				// send error to channel if it occurs
				if err != nil {
					errChan <- err
				}

				// add delay between batch requests if configured and if there are multiple workers
				if a.batchWorkers > 1 && a.batchdelay > 0 {
					time.Sleep(time.Duration(a.batchdelay) * time.Millisecond)
				}

				// mark the batch job as done
				wg.Done()
			}
		}()
	}

	// send batches to the channel and increment wait group counter
	for i, batch := range batches {

		wg.Add(1)

		// send batch job to the channel
		jobs <- BatchJob{
			Info: BatchInfo{
				Index: i + 1,
				Total: totalbatches,
				Size:  len(batch),
			},
			Data: batch,
		}
	}

	// close the Jobs channel
	close(jobs)

	// wait for all workers to finish
	wg.Wait()

	// =========================
	// FINISH logging ( with mutex )
	mu.Lock()
	a.writeOperationFinish(start)
	mu.Unlock()

	// close the ERROR channel and check for errors
	close(errChan)
	if len(errChan) > 0 {
		return fmt.Errorf("#Error: some batches failed")
	}

	return nil
}

// split divides a slice of items into batches of specified size.
func split(items []interface{}, size int) [][]interface{} {

	var batches [][]interface{}

	for i := 0; i < len(items); i += size {

		end := i + size
		if end > len(items) {
			end = len(items)
		}

		// append the batch to the list of batches
		batches = append(batches, items[i:end])
	}

	return batches
}

// processOneBatch creates a new payload with the batch of items and sends a POST request to the specified URL.
func processOneBatch(
	a *Api,
	original map[string]interface{},
	job BatchJob,
	itemsKey string,
	url string,
	mu *sync.Mutex,
) error {

	// create a new payload for the batch
	newPayload := make(map[string]interface{})

	// copy all fields except the items key to the new payload
	for k, v := range original {
		if k != itemsKey {
			newPayload[k] = v
		}
	}

	// add the batch of items to the new payload
	newPayload[itemsKey] = job.Data

	// marshal the new payload to JSON
	jsonData, err := json.Marshal(newPayload)
	if err != nil {
		fmt.Println("#Error: marshal:", err)
		return err
	}

	// send POST request with the batch data
	resp, err := a.doRequest("POST", url, jsonData)
	if err != nil {

		fmt.Println("#Error: request:", err)

		// log the error for the batch ( with mutex )
		mu.Lock()
		a.writeBatchLog("POST", url, jsonData, []byte(err.Error()), job.Info, "ERROR")
		mu.Unlock()

		return err
	}

	// ----
	// log the response for the batch ( with mutex )
	mu.Lock()
	a.writeBatchLog("POST", url, jsonData, resp, job.Info, "OK")
	mu.Unlock()

	return nil
}
