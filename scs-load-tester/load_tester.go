package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	flagALB         = flag.String("alb", "", "ALB DNS name")
	flagFlows       = flag.Int("flows", 200000, "Total number of checkout flows to run")
	flagConcurrency = flag.Int("concurrency", 100, "Number of concurrent workers")
	flagDebug       = flag.Bool("debug", false, "Enable debug logging")
)

// ===== Counters =====

var (
	totalFlows    int64
	authCount     int64
	declinedCount int64
	badReqCount   int64
	otherErrors   int64

	// NEW: status code counter map
	statusMu      sync.Mutex
	statusCountMap = map[int]int64{}

	// NEW: capture sample client errors
	errMu      sync.Mutex
	errSamples []string
	errLimit   = 20
)

// ===== Helper =====

// correct version — record status codes
func recordStatus(code int) {
	statusMu.Lock()
	statusCountMap[code]++
	statusMu.Unlock()
}

func recordClientErr(err error) {
	errMu.Lock()
	if len(errSamples) < errLimit {
		errSamples = append(errSamples, err.Error())
	}
	errMu.Unlock()
}

type createCartResp struct {
	ShoppingCartID int `json:"shopping_cart_id"`
}

func newHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}
}

func doJSONRequest(client *http.Client, method, url string, body any) (*http.Response, []byte, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		buf = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	return resp, respBody, nil
}

func runFlow(client *http.Client, baseURL string, debug bool) {
	atomic.AddInt64(&totalFlows, 1)

	// 1. create cart
	resp, body, err := doJSONRequest(client, "POST", baseURL+"/shopping-cart", map[string]any{
		"customer_id": 1,
	})
	if err != nil {
		recordClientErr(err)
		atomic.AddInt64(&otherErrors, 1)
		return
	}

	if resp.StatusCode != http.StatusCreated {
		recordStatus(resp.StatusCode)
		atomic.AddInt64(&otherErrors, 1)
		if debug {
			fmt.Printf("[create-cart] %d %s\n", resp.StatusCode, string(body))
		}
		return
	}

	var cart createCartResp
	if err := json.Unmarshal(body, &cart); err != nil || cart.ShoppingCartID == 0 {
		atomic.AddInt64(&otherErrors, 1)
		return
	}

	// 2. add item
	addURL := fmt.Sprintf("%s/shopping-carts/%d/addItem", baseURL, cart.ShoppingCartID)
	resp, body, err = doJSONRequest(client, "POST", addURL, map[string]any{
		"product_id": 1,
		"quantity":   1,
	})
	if err != nil {
		recordClientErr(err)
		atomic.AddInt64(&otherErrors, 1)
		return
	}
	if resp.StatusCode != http.StatusNoContent {
		recordStatus(resp.StatusCode)
		atomic.AddInt64(&otherErrors, 1)
		return
	}

	// 3. checkout
	ckURL := fmt.Sprintf("%s/shopping-carts/%d/checkout", baseURL, cart.ShoppingCartID)
	resp, body, err = doJSONRequest(client, "POST", ckURL, map[string]any{
		"credit_card_number": "1234-5678-9012-3456",
	})
	if err != nil {
		recordClientErr(err)
		atomic.AddInt64(&otherErrors, 1)
		return
	}

	switch resp.StatusCode {
	case 200:
		atomic.AddInt64(&authCount, 1)
	case 402:
		atomic.AddInt64(&declinedCount, 1)
	case 400:
		atomic.AddInt64(&badReqCount, 1)
	default:
		recordStatus(resp.StatusCode)
		atomic.AddInt64(&otherErrors, 1)
		if debug {
			fmt.Printf("[checkout] %d %s\n", resp.StatusCode, string(body))
		}
	}
}

func main() {
	flag.Parse()

	if *flagALB == "" {
		fmt.Println("Usage: go run load_tester.go -alb <ALB> [-flows 200000] [-concurrency 100] [-debug]")
		os.Exit(1)
	}

	baseURL := "http://" + *flagALB
	total := *flagFlows
	conc := *flagConcurrency
	debug := *flagDebug

	fmt.Println("===== Load Tester =====")
	fmt.Println("ALB:", *flagALB)
	fmt.Println("Flows:", total)
	fmt.Println("Concurrency:", conc)
	fmt.Println("Debug:", debug)

	client := newHTTPClient()
	start := time.Now()

	taskCh := make(chan struct{})
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range taskCh {
				runFlow(client, baseURL, debug)
			}
		}()
	}

	// Producer
	go func() {
		for i := 0; i < total; i++ {
			taskCh <- struct{}{}
		}
		close(taskCh)
	}()

	wg.Wait()

	elapsed := time.Since(start).Seconds()

	fmt.Println("===== Results =====")
	fmt.Printf("Total flows requested: %d\n", total)
	fmt.Printf("Total flows finished : %d\n", totalFlows)
	fmt.Printf("200 Authorized     : %d\n", authCount)
	fmt.Printf("402 Declined       : %d\n", declinedCount)
	fmt.Printf("400 Bad Request    : %d\n", badReqCount)
	fmt.Printf("Other errors       : %d\n", otherErrors)
	fmt.Printf("Elapsed time       : %.2fs\n", elapsed)
	fmt.Printf("Throughput         : %.2f req/s\n", float64(totalFlows)/elapsed)

	if debug {
		// print status breakdown
		fmt.Println("----- Status breakdown -----")
		statusMu.Lock()
		keys := []int{}
		for k := range statusCountMap {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			fmt.Printf("%d : %d\n", k, statusCountMap[k])
		}
		statusMu.Unlock()

		// print client-side errors
		errMu.Lock()
		if len(errSamples) > 0 {
			fmt.Println("----- Sample client-side errors -----")
			for i, e := range errSamples {
				fmt.Printf("[%d] %s\n", i+1, e)
			}
		}
		errMu.Unlock()
	}
}