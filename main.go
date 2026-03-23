package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

const (
	outputFile    = "output.csv"
	inputFile     = "input.csv"
	objectFile    = "object.csv"
	outputFileXml = "output.xml"
	inputFileXml  = "input.xml"
)

type Node struct {
	XMLName xml.Name
	Nodes   []Node `xml:",any,omitempty"`
	Value   string `xml:",chardata"`
}

type ApiResponse struct {
	//Success bool   	`json:"success"`
	//Message string 	`json:"message"`
	//Errors  []string	`json:"errors"`
	Data []map[string]interface{} `json:"data"`
	Meta PageData                 `json:"meta"`
}

type PageData struct {
	Page  int `json:"current_page"`
	Total int `json:"last_page"`
}

type RequestXML struct {
	XMLName xml.Name `xml:"request"`
	Meta    Node     `xml:"meta"`
	Data    Node     `xml:"data"`
}
type Api struct {
	url         string
	apiURL      string
	inputPath   string
	outputPath  string
	token       string
	debug       bool
	xml         bool
	client      *http.Client
	responseLog *os.File
}

func NewApi(baseURL, apiURL string, conf Config, xml bool) *Api {

	api := &Api{
		url:        buildURL(baseURL, apiURL),
		apiURL:     apiURL,
		inputPath:  conf.InputPath,
		outputPath: conf.OutputPath,
		token:      conf.BearerToken,
		xml:        xml,
		client:     newHTTPClient(),
	}

	if api.token == "" {
		fmt.Println("#Warn: no token provided")
	}

	return api
}

func main() {

	// program start
	fmt.Println("...Starting Api Http Caller v1.2.5 (c)")
	StartTime := time.Now()

	// command line flags
	configPath := flag.String("conf", "config.yml", "path to config file")
	apiURL := flag.String("url", "", "API resource URL to fetch data from")
	apiMethod := flag.String("method", "GET", "HTTP method (GET, POST, etc.)")
	workPath := flag.String("path", "", "working directory")
	boundary := flag.String("boundary", "", "File name to be send using boundary")
	debug := flag.Bool("debug", false, "enable debug mode")
	apiXml := flag.Bool("xml", false, "enable xml mode")
	flag.Parse()

	// validate required parameters
	if *apiURL == "" {
		fmt.Println("Please provide an API URL.")
		return
	}

	// read configuration file
	conf, err := GetConfig(*configPath)
	if err != nil {
		fmt.Println("reading config file:", err)
		return
	}

	// validate base URL
	baseUrl := conf.BaseUrl
	if baseUrl == "" {
		fmt.Println("Please provide a base URL in the configuration file.")
		return
	}

	// initialize Api struct with configuration and command line parameters
	api := NewApi(baseUrl, *apiURL, *conf, *apiXml)

	// override paths if provided via command line
	if workPath != nil && *workPath != "" {
		api.inputPath = *workPath
		api.outputPath = *workPath
	}

	// debug output
	if *debug {

		fmt.Println("Debug mode is ON")
		fmt.Println("Config file:", *configPath)
		fmt.Println("API URL:", *apiURL)
		fmt.Println("API Method:", *apiMethod)
		fmt.Println("Working directory:", *workPath)
		fmt.Println("Boundary:", *boundary)
		fmt.Println("XML mode:", *apiXml)
		api.debug = true
	}

	// setup logging to file
	logFile := fmt.Sprintf("%serrors.log", api.outputPath)

	// remove previous log file if exists (optional, can be commented out if you want to keep logs from previous runs)
	//_ = os.Remove(logFile)

	// check log file size and rotate if it exceeds the specified maximum size
	rotateLogIfNeeded(logFile, 50*1024*1024) // 50MB

	// cleanup old log files, keeping only the 5 most recent ones based on modification time
	cleanupOldLogs(api.outputPath, "errors.log", 5) // keep only 5 logs

	// open log file for appending, create if not exists
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("opening or creating log file: %v\n", err)
		return
	}

	// ensure log file is closed at the end
	defer func(f *os.File) {

		// log finish time
		fmt.Printf("Finished in %s\n", time.Since(StartTime))
		fmt.Printf("Finished ===================================== <<< %s\n\n", logTime(time.Now()))

		// close log file
		err = f.Close()
		if err != nil {
			fmt.Println("closing log file:", err)
			return
		}
	}(file)

	// redirect standard error to the log file
	responseLogPath := fmt.Sprintf("%sresponse.log", api.outputPath)

	// remove previous log file if exists (optional, can be commented out if you want to keep logs from previous runs)
	//_ = os.Remove(responseLogPath)

	// check log file size and rotate if it exceeds the specified maximum size
	rotateLogIfNeeded(responseLogPath, 50*1024*1024) // 50MB

	// cleanup old log files, keeping only the 5 most recent ones based on modification time
	cleanupOldLogs(api.outputPath, "response.log", 5) // keep only 5 logs

	// open response log file for appending, create if not exists
	respFile, err := os.OpenFile(responseLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("opening response log:", err)
		return
	}

	// ensure response log file is closed at the end
	api.responseLog = respFile

	//
	defer func(f *os.File) {
		// close log file
		err = f.Close()
		if err != nil {
			fmt.Println("closing response log file:", err)
			return
		}
	}(respFile)

	// redirect standard output to the log file
	os.Stdout = file

	// log start time
	fmt.Println("Start ===================================== >>>", logTime(StartTime))

	// remove previous output files
	api.removeFiles()

	// determine HTTP method
	method := strings.ToUpper(*apiMethod)

	// handle multipart POST if boundary is provided only for POST method and csv file (xml multipart is not implemented in this version)
	if boundary != nil && *boundary != "" && method == "POST" {
		api.doMultipartPost(*boundary)
		return
	}

	// for GET no body
	var jsonBytes []byte

	// for other methods prepare body
	if method != "GET" {

		// read input file(s) and prepare JSON body
		if api.xml {
			jsonBytes, err = prepareBodyXML(api.inputPath)
		} else {
			jsonBytes, err = prepareBody(api.inputPath)
		}

		// handle error
		if err != nil {
			fmt.Println("#Error: preparing body:", err)
			return
		}
	}

	// determine output file name
	outputFileName := outputFile
	if api.xml {
		outputFileName = outputFileXml
	}

	// make the HTTP call
	api.doHttpMethod(method, api.url, jsonBytes, outputFileName)

}

// newHTTPClient - creates a new HTTP client with a timeout
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 90 * time.Second,
	}
}

// HasNextPage - checks if there are more pages to fetch based on the current page and total pages
func (p PageData) HasNextPage() bool {
	return p.Page > 0 && p.Total > p.Page
}

// NextPage - calculates the next page number
func (p PageData) NextPage() int {
	return p.Page + 1
}

func buildURL(baseURL, apiURL string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(apiURL, "/")
}

// logTime - formats time for logging
func logTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.000")
}

// cleanupOldLogs - removes old log files, keeping only the specified maximum number of recent log files based on modification time
func cleanupOldLogs(dir, baseName string, maxFiles int) {

	// read all files in the log directory
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("#Error: reading log dir:", err)
		return
	}

	// struct to hold log file information
	type fileInfo struct {
		name string
		time time.Time
	}

	var logs []fileInfo

	// collect log files with their modification times,
	// skipping the current log file and directories
	for _, f := range files {

		if f.IsDir() {
			continue
		}

		// we look for files that start with the base name (e.g., "errors.log")
		// to identify rotated log files like "errors.log.20230601_120000"
		name := f.Name()

		if name == baseName {
			continue // skip current log file
		}

		if strings.HasPrefix(name, baseName) {

			info, err := f.Info()
			if err != nil {
				continue
			}

			logs = append(logs, fileInfo{
				name: name,
				time: info.ModTime(),
			})
		}
	}

	// sort log files by modification time in descending order (newest first)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].time.After(logs[j].time)
	})

	// if the number of log files is within the limit,
	//  we can return without deleting anything
	if len(logs) <= maxFiles {
		return
	}

	// 	delete old log files, keeping only the most recent ones based on modification time
	for i := maxFiles; i < len(logs); i++ {

		path := filepath.Join(dir, logs[i].name)

		err := os.Remove(path)
		if err != nil {
			fmt.Println("#Error: removing old log:", err)
		} else {
			fmt.Println("[INFO] removed old log:", logs[i].name)
		}
	}
}

// rotateLogIfNeeded - checks the size of the log file and rotates it if it exceeds the specified maximum size
func rotateLogIfNeeded(path string, maxSize int64) {

	info, err := os.Stat(path)
	if err != nil {
		return // if the file does not exist
	}

	if info.Size() < maxSize {
		return
	}

	// имя архива
	timestamp := time.Now().Format("20060102_150405")
	newName := fmt.Sprintf("%s.%s", path, timestamp)

	err = os.Rename(path, newName)
	if err != nil {
		fmt.Println("#Error: rotate log:", err)
		return
	}
}

// writeResponseLog - writes the request and response details to the response log file for debugging purposes
func (a *Api) writeResponseLog(method, url string, requestBody, responseBody []byte) {

	if a.responseLog == nil {
		return
	}

	StartTime := time.Now()

	a.responseLog.WriteString("=====================================================\n")
	a.responseLog.WriteString(fmt.Sprintf("Start: %s\n\n", logTime(StartTime)))

	// Request
	a.responseLog.WriteString("Request ===================================== >>>\n")
	a.responseLog.WriteString(fmt.Sprintf("%s %s\n", method, url))

	if len(requestBody) > 0 {
		a.responseLog.Write(requestBody)
		a.responseLog.WriteString("\n")
	}

	a.responseLog.WriteString("Request ===================================== <<<\n\n")

	// Response
	a.responseLog.WriteString("Response ===================================== >>>\n")

	_, err := a.responseLog.Write(responseBody)
	if err != nil {
		fmt.Println("#Error: writing response log:", err)
	}

	a.responseLog.WriteString("\nResponse ===================================== <<<\n\n")

	a.responseLog.WriteString(fmt.Sprintf("Finished in %s\n", time.Since(StartTime)))
	a.responseLog.WriteString(fmt.Sprintf("Finished: %s\n", logTime(time.Now())))
	a.responseLog.WriteString("=====================================================\n")

	// flush the log to ensure it's written to disk
	a.responseLog.Sync()
}

// safeName - converts a string to a safe XML tag name by replacing invalid characters with underscores and ensuring it doesn't start with a digit
func safeName(s string) string {

	s = strings.TrimSpace(s)

	var result []rune

	for _, r := range s {

		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			result = append(result, r)
		} else {
			result = append(result, '_')
		}
	}

	if len(result) == 0 {
		return "Field"
	}

	if unicode.IsDigit(result[0]) {
		return "F_" + string(result)
	}

	return string(result)
}

// toXML - converts a Node to an indented string
func NodetoXML(node Node, indent string) string {

	if len(node.Nodes) == 0 {
		return fmt.Sprintf("%s<%s>%s</%s>\n", indent, node.XMLName.Local, node.Value, node.XMLName.Local)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s<%s>\n", indent, node.XMLName.Local))
	for _, child := range node.Nodes {
		sb.WriteString(NodetoXML(child, indent+"  "))
	}
	sb.WriteString(fmt.Sprintf("%s</%s>\n", indent, node.XMLName.Local))
	return sb.String()
}

// Convert the array name to a singular number
func singularName(name string) string {

	// For words ending in "s" (but not "ss"), we remove the trailing "s" to get the singular form.
	if strings.HasSuffix(name, "s") && len(name) > 1 {
		return strings.TrimSuffix(name, "s")
	}

	return "item"
}

// extractServiceTag - extracts service tag from URL parameter
func extractServiceTag(urlParam string) string {

	// urlParam = "/orders?page=2&date="
	// remove the leading "/"
	urlPath := strings.TrimPrefix(urlParam, "/")

	// We look for the "?" symbol and cut off everything after it.
	if idx := strings.Index(urlPath, "?"); idx != -1 {
		urlPath = urlPath[:idx]
	}

	// convert to a safe XML name
	return safeName(urlPath)
}

// convertJSONtoXML - recursively converts JSON-like data to an XML tree of Nodes
func convertJSONtoXML(name string, v interface{}) Node {

	// create a new Node with the given name, ensuring it's safe for XML
	node := Node{
		XMLName: xml.Name{Local: safeName(name)},
	}

	// simple fields at the top, arrays/objects at the bottom
	var childrenValues []Node
	var childrenArrays []Node

	// determine the type of the value and process accordingly
	switch val := v.(type) {

	// if it's a map, we treat it as an object and recursively convert its fields to child Nodes
	case map[string]interface{}:

		// 🔹 Collect and sort keys to make XML deterministic
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}

		// 🔹 Sort keys alphabetically for consistent output
		sort.Strings(keys)

		// 🔹 Process in stable order
		for _, k := range keys {

			// recursively convert each field to XML
			child := convertJSONtoXML(k, val[k])

			// objects/arrays go to bottom, simple values at the top
			if len(child.Nodes) > 0 && child.XMLName.Local != "" {
				childrenArrays = append(childrenArrays, child)
			} else {
				childrenValues = append(childrenValues, child)
			}
		}

	// if it's an array, we treat it as a collection and recursively convert each item to a child Node with the singular form of the name
	case []interface{}:

		// 🔹 For arrays, we use the singular form of the name for each item
		itemName := singularName(name)
		for _, v := range val {

			// recursively convert each item to XML and add to children arrays
			childrenArrays = append(childrenArrays, convertJSONtoXML(itemName, v))
		}

	case nil:
		node.Value = ""

	case bool:
		if val {
			node.Value = "true"
		} else {
			node.Value = "false"
		}

	default:
		node.Value = fmt.Sprint(val)
	}

	// 🔹 Final layout: values first, collections after
	node.Nodes = append(childrenValues, childrenArrays...)

	return node
}

// doHttpMethod - performs the HTTP request with the given method, URL, and body,
// then processes the response and handles pagination if necessary
func (a *Api) doHttpMethod(method string, requestURL string, data []byte, output string) {

	// log the request being made
	fmt.Printf("%s: %s\n", method, requestURL)

	//	create HTTP request
	req, err := http.NewRequest(method, requestURL, bytes.NewBuffer(data))
	if err != nil {
		fmt.Println("#Error: creating request:", err)
		return
	}

	// set headers
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.token))
	}

	// make the HTTP request
	resp, err := a.client.Do(req)
	if err != nil {
		fmt.Println("#Error: making request:", err)
		return
	}

	// ensure response body is closed at the end
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			fmt.Println("#Error: closing response body:", err)
			return
		}
	}(resp.Body)

	// check for non-successful status codes
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("#Error: reading response body:", err)
		return
	}

	// log the request and response details to the response log file for debugging purposes
	a.writeResponseLog(method, requestURL, data, body)

	// log the request and response details to the response log file for debugging purposes
	if a.debug {
		fmt.Printf("[DEBUG] response size: %d bytes\n", len(body))
	}
	//
	var apiResponse ApiResponse
	//err = json.Unmarshal(body, &apiResponse)
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	err = dec.Decode(&apiResponse)

	// if there was an error parsing the response as JSON, we log a warning and attempt to save the raw response as XML
	// if it's a GET request and XML mode is enabled, otherwise we just log the error and return
	if err != nil {

		// if the response cannot be parsed as JSON, we log a warning and attempt to save the raw response as XML if
		// it's a GET request and XML mode is enabled, otherwise we just log the error and return
		if strings.EqualFold(method, "GET") && a.xml {

			if a.debug {
				fmt.Println("#Info: response not matching ApiResponse structure - save the raw response in xml if possible")
			}
			a.saveRawResponseXML(body, output)
		}
		return
	}

	// if the response does not contain a "data" field, we log a warning and attempt to save the raw response as XML
	// if it's a GET request and XML mode is enabled, otherwise we just return
	if apiResponse.Data == nil {

		// if the response does not contain a "data" field, we save the raw response for GET requests
		// to avoid losing potentially important information
		if strings.EqualFold(method, "GET") && a.xml {

			if a.debug {
				fmt.Println("#Info: response not matching ApiResponse structure - save the raw response in xml if possible")
			}
			a.saveRawResponseXML(body, output)
		}
		return
	}

	if len(apiResponse.Data) == 0 {
		if a.debug {
			fmt.Println("#Info: empty data array")
		}
		return
	}

	// save in CSV or XML
	if a.xml {
		a.saveResponseXml(apiResponse, output)
	} else {
		a.saveResponse(apiResponse, output)
	}

	//fmt.Println("#apiResponse.Meta.Total:", apiResponse.Meta.Total)
	//fmt.Println("#apiResponse.Meta.Page:", apiResponse.Meta.Page)

	// if there are more pages to fetch, we calculate the next page URL and recursively call doHttpMethod for the next page
	meta := apiResponse.Meta
	if meta.HasNextPage() {

		// calculate the next page number and log it
		nextPage := meta.NextPage()
		fmt.Printf("fetching page %d of %d...\n", nextPage, meta.Total)

		parsedParams, err := url.Parse(requestURL)
		if err != nil {
			fmt.Println("#Error: parsing URL:", err)
			return
		}

		// update the "page" query parameter
		params := parsedParams.Query()
		params.Set("page", fmt.Sprintf("%d", nextPage))
		parsedParams.RawQuery = params.Encode()

		// get the next URL with the updated page parameter
		nextURL := parsedParams.String()

		// determine output file extension
		extFileOutput := "csv"
		if a.xml {
			extFileOutput = "xml"
		}

		// recursively call doHttpMethod for the next page
		a.doHttpMethod("GET", nextURL, nil, fmt.Sprintf("output_%d.%s", nextPage, extFileOutput))
	}
}

// saveRawResponse - saves the raw API response body to a file, attempting to convert JSON to XML if possible
func (a *Api) saveRawResponseXML(body []byte, output string) {

	fileName := filepath.Join(a.outputPath, output)

	// decode generic JSON
	obj, err := DecodeJSON(body)
	if err != nil {

		// if it's not JSON, log a warning and save the raw response
		fmt.Println("#Warn: response is not JSON, saving raw")

		// if it's not JSON, just save the raw response
		writeAtomic(fileName, body)
		return
	}

	if obj == nil {
		if a.debug {
			fmt.Println("#Info: empty JSON response, skipping file")
		}
		return
	}

	if len(obj) == 0 {
		if a.debug {
			fmt.Println("#Info: empty JSON object, skipping file")
		}
		return
	}

	// root tag based on service
	serviceName := extractServiceTag(a.apiURL)
	rootTag := safeName(serviceName)

	// attempt to convert JSON to XML and save, if it fails we will have already saved the raw response above, so we can just log the error and return
	a.writeXMLFile(rootTag, obj, output)

	fmt.Printf("raw JSON converted to XML: %s\n", fileName)
}

// saveResponseXml - saves the API response data as an XML file
func (a *Api) saveResponseXml(response ApiResponse, output string) {

	fileName := filepath.Join(a.outputPath, output)

	if len(response.Data) == 0 {
		fmt.Println("#Warn: no data to write")
		return
	}

	items := make([]interface{}, len(response.Data))
	for i := range response.Data {
		items[i] = response.Data[i]
	}

	serviceName := extractServiceTag(a.apiURL)
	rootTag := safeName(serviceName)

	// attempt to convert JSON to XML and save, if it fails we will have already saved the raw response above, so we can just log the error and return
	a.writeXMLFile(rootTag, items, output)

	// success message
	fmt.Printf(
		"received %d records (xml) -> %s\n",
		len(response.Data),
		fileName,
	)
}

func (a *Api) writeXMLFile(rootName string, data interface{}, output string) {

	fileName := filepath.Join(a.outputPath, output)

	root := convertJSONtoXML(rootName, data)

	xmlBody, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		fmt.Println("#Error: marshal xml:", err)
		return
	}

	xmlFull := append(
		[]byte(`<?xml version="1.0" encoding="windows-1251"?>`+"\n"),
		xmlBody...,
	)

	encoder := charmap.Windows1251.NewEncoder()
	cp1251Data, err := encoder.Bytes(xmlFull)
	if err != nil {
		fmt.Println("#Error: encoding windows-1251:", err)
		return
	}

	writeAtomic(fileName, cp1251Data)
}

// saveResponse - saves the API response data as a CSV file
func (a *Api) saveResponse(response ApiResponse, output string) {

	//if !response.Success {
	//	fmt.Println("#Error: call was not successful")
	//	return
	//}

	// Create CSV file
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	if len(response.Data) == 0 {
		fmt.Println("#Warn: no data to write")
		return
	}

	// Write header
	var header []string
	for key := range response.Data[0] {
		header = append(header, key)
	}

	err := writer.Write(header)
	if err != nil {
		fmt.Println("#Error: writing header:", err)
		return
	}

	// Write data rows
	for _, row := range response.Data {
		var record []string
		for _, key := range header {
			value := fmt.Sprintf("%v", row[key])
			value = strings.ReplaceAll(value, "\n", " ")
			value = strings.ReplaceAll(value, "\r", "")
			encoded, e := ConvertToWindows1251(value)
			if a.debug && e != nil {
				fmt.Printf("#Error: converting string: %s\n", e)
				fmt.Printf("#Error: failed to convert: %s\n", value)
			}
			record = append(record, encoded)
		}
		err := writer.Write(record)
		if err != nil {
			fmt.Println("#Error: writing record:", err)
			return
		}
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		fmt.Println("#Error: flushing csv:", err)
		return
	}

	// write to file
	fileName := fmt.Sprintf("%s%s", a.outputPath, output)

	// write atomically to avoid partial file issues
	writeAtomic(fileName, buf.Bytes())

	// success message
	if buf.Len() == 0 {
		fmt.Println("#Warn: empty csv generated")
	}

	fmt.Printf("received %d records: %s\n", len(response.Data), output)
}

// getChildXML - retrieves a child Node by name
func getChildXML(node Node, name string) *Node {

	for i, child := range node.Nodes {
		if child.XMLName.Local == name {
			return &node.Nodes[i]
		}
	}
	return nil
}

// XMLnodeToInterface - converts an XML Node to a generic interface{}
func XMLnodeToInterface(n Node) interface{} {

	// ✅ Value
	if len(n.Nodes) == 0 {
		return strings.TrimSpace(n.Value)
	}

	// if this as a array or object
	sameName := true
	first := n.Nodes[0].XMLName.Local

	for _, ch := range n.Nodes {
		if ch.XMLName.Local != first {
			sameName = false
			break
		}
	}

	// ✅ Array
	if sameName {
		arr := make([]interface{}, 0, len(n.Nodes))
		for _, ch := range n.Nodes {
			arr = append(arr, XMLnodeToInterface(ch))
		}
		return arr
	}

	// ✅ Object
	obj := make(map[string]interface{})
	for _, ch := range n.Nodes {
		obj[ch.XMLName.Local] = XMLnodeToInterface(ch)
	}

	return obj
}

// dataNodeToPayload - converts the data Node to a payload map
func dataNodeToPayload(data Node) map[string]interface{} {

	result := make(map[string]interface{})

	for _, child := range data.Nodes {
		result[child.XMLName.Local] = XMLnodeToInterface(child)
	}

	return result
}

// getJsonBytesFromXMLData - converts XML data Node to JSON bytes
func getJsonBytesFromXMLData(dataNode Node) ([]byte, error) {

	// converting xml to universal structure
	payload := dataNodeToPayload(dataNode)

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON from XML: %w", err)
	}

	fmt.Printf("[DEBUG] request body size: %d bytes\n", len(jsonBytes))

	return jsonBytes, nil
}

// prepareBodyXML - prepares JSON body from XML input file
func prepareBodyXML(path string) ([]byte, error) {

	// single XML file
	reqXML, err := readFileContentXML(path, inputFileXml)

	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", inputFileXml, err)
	}

	// service - url
	var pservice, purl string
	if n := getChildXML(reqXML.Meta, "service"); n != nil {
		pservice = n.Value
	}
	if n := getChildXML(reqXML.Meta, "url"); n != nil {
		purl = n.Value
	}

	fmt.Println("Service -", pservice, "URL-", purl)

	// getting data Node
	dataNode := reqXML.Data

	// converting data Node to JSON bytes
	jsonBytes, err := getJsonBytesFromXMLData(dataNode)
	if err != nil {
		return nil, fmt.Errorf("reading file content: %s: %w", inputFileXml, err)
	}

	return jsonBytes, nil
}

// prepareBody - prepares JSON body from input file(s)
func prepareBody(path string) ([]byte, error) {

	singleFile, err := readFileContent(path, objectFile)
	if err == nil {
		if len(singleFile) > 0 {
			obj := singleFile[0]
			return getJsonBytes(obj)
		}
		return nil, fmt.Errorf("empty object data file")
	}

	singleFile, err = readFileContent(path, inputFile)
	if err == nil {
		return getJsonBytes(singleFile)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %s: %s", path, err)
	}

	result := make(map[string][]map[string]interface{})

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "input_") && strings.HasSuffix(file.Name(), ".csv") {

			jsonPayload, err := readFileContent(path, file.Name())
			if err != nil {
				return nil, fmt.Errorf("reading file content: %s: %w", file.Name(), err)
			}

			keyName := strings.TrimPrefix(file.Name(), "input_")
			keyName = strings.TrimSuffix(keyName, ".csv")

			result[keyName] = jsonPayload
		}
	}

	return getJsonBytes(result)
}

// readFileContentXML - reads XML file and parses it into RequestXML struct
func readFileContentXML(path, fileName string) (*RequestXML, error) {

	filePath := filepath.Join(path, fileName)

	dataBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", fileName, err)
	}

	fmt.Println("Reading XML file:", fileName)

	var req RequestXML

	decoder := xml.NewDecoder(bytes.NewReader(dataBytes))
	decoder.CharsetReader = charset.NewReaderLabel

	if err := decoder.Decode(&req); err != nil {
		return nil, fmt.Errorf("parsing XML: %w", err)
	}
	// alternative simpler way (to be used if no special charset windows-1251 handling is needed)
	//if err := xml.Unmarshal(dataBytes, &req); err != nil {
	//	return nil, fmt.Errorf("parsing XML: %w", err)
	//}

	return &req, nil
}

// readFileContent - reads CSV file and converts it to a slice of maps
func readFileContent(path, fileName string) ([]map[string]interface{}, error) {

	filePath := filepath.Join(path, fileName)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %s: %s", fileName, err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("#Error: closing file:", err)
			return
		}
	}(file)

	fmt.Println("Reading file:", fileName)

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var jsonPayload []map[string]interface{}
	header := records[0]
	for _, row := range records[1:] {
		var record = make(map[string]interface{})
		for i, key := range header {
			field, err := ConvertToUTF8(row[i])
			if err != nil {
				fmt.Println("#Error: converting to utf-8:", err)
			}
			record[key] = field
		}
		jsonPayload = append(jsonPayload, record)
	}

	return jsonPayload, nil
}

// getJsonBytes - converts a generic interface to JSON bytes
func getJsonBytes(v any) ([]byte, error) {

	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON: %w", err)
	}

	fmt.Printf("[DEBUG] request body size: %d bytes\n", len(jsonBytes))

	return jsonBytes, nil
}

// ConvertToUTF8 - converts a Windows-1251 encoded string to UTF-8
func ConvertToUTF8(win1251 string) (string, error) {

	decoder := charmap.Windows1251.NewDecoder()
	utf8Content, err := decoder.String(win1251)
	if err != nil {
		return "", err
	}
	return utf8Content, nil
}

// ConvertToWindows1251 - converts a UTF-8 encoded string to Windows-1251, replacing unsupported characters with space and collapsing multiple spaces into one
func ConvertToWindows1251(utf8Str string) (string, error) {
	enc := encoding.ReplaceUnsupported(charmap.Windows1251.NewEncoder())
	win1251Content, err := enc.String(utf8Str)
	if err != nil {
		return "", err
	}

	// replace all '?' (replacement) with space
	win1251Content = strings.ReplaceAll(win1251Content, "?", " ")

	// collapse multiple spaces into one
	win1251Content = strings.Join(strings.FieldsFunc(win1251Content, unicode.IsSpace), " ")

	return win1251Content, nil
}

// removeFiles - removes previously generated output files to avoid confusion with new results
func (a *Api) removeFiles() {

	files, err := filepath.Glob(filepath.Join(a.outputPath, "output*"))
	if err != nil {
		fmt.Println("reading directory:", err)
		return
	}

	for _, file := range files {

		if strings.HasSuffix(file, ".csv") ||
			strings.HasSuffix(file, ".xml") ||
			strings.HasSuffix(file, ".tmp") {

			err := os.Remove(file)
			if err != nil {
				fmt.Printf("deleting file %s: %v\n", file, err)
			}
		}
	}
}

// doMultipartPost - performs a multipart/form-data POST request with a file
func (a *Api) doMultipartPost(boundary string) {
	fmt.Printf("POST: %s\n", a.url)

	file, err := os.Open(fmt.Sprintf("%s%s", a.inputPath, boundary))
	if err != nil {
		fmt.Println("#Error: opening file:", err)
		return
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			fmt.Println("#Error: closing file:", err)
			return
		}
	}(file)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", boundary)
	if err != nil {
		fmt.Println("#Error: creating form file:", err)
		return
	}

	_, err = io.Copy(part, file)
	if err != nil {
		fmt.Println("#Error: copying file to form file:", err)
		return
	}

	err = writer.Close()
	if err != nil {
		fmt.Println("#Error: closing writer:", err)
		return
	}

	//fmt.Println("Body ===================================== >>>")
	//fmt.Printf("%s\n", body)
	//fmt.Println("Body ===================================== <<<")

	if a.debug {
		fmt.Printf("[DEBUG] request body size: %d bytes\n", len(body.Bytes()))
	}

	req, err := http.NewRequest("POST", a.url, body)
	if err != nil {
		fmt.Println("#Error: creating request:", err)
		return
	}

	// set headers
	content := writer.FormDataContentType()
	fmt.Println("Content-Type:", content)
	req.Header.Set("Content-Type", content)

	// make the HTTP request
	resp, err := a.client.Do(req)
	if err != nil {
		fmt.Println("#Error: making request:", err)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("#Error: closing response body:", err)
			return
		}
	}(resp.Body)

	if resp.StatusCode > 299 {
		fmt.Printf("#Error: response status %s\n", resp.Status)
	}

	//response, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	fmt.Println("#Error: reading response body:", err)
	//	return
	//}
	//
	//fmt.Println("Response ===================================== >>>")
	//fmt.Printf("%s\n", string(response))
	//fmt.Println("Response ===================================== <<<")
}

// DecodeJSON parses a JSON-encoded byte slice into a generic interface and validates that the top-level object is a map.
// It converts JSON numbers to int64 or float64 where applicable for numeric accuracy. Returns the parsed object or an error.
func DecodeJSON(body []byte) (map[string]interface{}, error) {

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}

	obj, ok := normalizeNumbers(v).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected top-level JSON object, got %T", v)
	}
	return obj, nil
}

// normalizeNumbers converts json.Number types to int64 or float64 where applicable, preserving the structure of the input.
func normalizeNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, val := range x {
			m[k] = normalizeNumbers(val)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, val := range x {
			s[i] = normalizeNumbers(val)
		}
		return s
	case json.Number:
		// Сначала пытаемся как целое
		if i, err := x.Int64(); err == nil {
			return i
		}
		// Иначе как float64 (учти возможную потерю точности у очень больших чисел)
		if f, err := x.Float64(); err == nil {
			return f
		}
		// Фолбэк — оставить строкой
		return x.String()
	default:
		return v
	}
}

// writeAtomic writes data to a temporary file and then atomically renames it
func writeAtomic(path string, data []byte) {

	// create a temporary file pathby appending ".tmp" to the target file name
	tmp := path + ".tmp"

	//	create temp file
	file, err := os.Create(tmp)
	if err != nil {
		fmt.Println("#Error: creating temp file:", err)
		return
	}

	// write data to temp file
	_, err = file.Write(data)
	if err != nil {
		fmt.Println("#Error: writing temp file:", err)
		file.Close()
		return
	}

	// ensure data is flushed to disk before renaming
	err = file.Sync()
	if err != nil {
		fmt.Println("#Error: syncing temp file:", err)
		file.Close()
		return
	}

	// close temp file before renaming
	err = file.Close()
	if err != nil {
		fmt.Println("#Error: closing temp file:", err)
		return
	}

	// rename temp file to final path
	err = os.Rename(tmp, path)
	if err != nil {
		fmt.Println("#Error: renaming temp file:", err)
		return
	}

	// fsync directory (crash-safe rename)
	dir := filepath.Dir(path)
	d, err := os.Open(dir)
	if err == nil {
		d.Sync()
		d.Close()
	}
}
