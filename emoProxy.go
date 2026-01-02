package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type emo_time struct {
	Time   int64 `json:"time"`
	Offset int   `jason:"offset"`
}

type emo_code struct {
	Code       int64  `json:"code"`
	Errmessage string `json:"errmessage"`
}

// --- Support for QueryResponse override logic ---

type Intent struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

// BehaviorParas supports both flat and nested structures.
type BehaviorParas struct {
	UtilityType   string         `json:"utility_type,omitempty"`
	Time          []string       `json:"time,omitempty"`
	Txt           string         `json:"txt,omitempty"`
	Url           string         `json:"url,omitempty"`
	PreAnimation  string         `json:"pre_animation,omitempty"`
	PostAnimation string         `json:"post_animation,omitempty"`
	PostBehavior  string         `json:"post_behavior,omitempty"`
	RecBehavior   string         `json:"rec_behavior,omitempty"`
	BehaviorParas *BehaviorParas `json:"behavior_paras,omitempty"`
	Sentiment     string         `json:"sentiment,omitempty"`
	Listen        int            `json:"listen,omitempty"`
}

type QueryResult struct {
	ResultCode    string        `json:"resultCode"`
	QueryText     string        `json:"queryText"`
	Intent        Intent        `json:"intent"`
	RecBehavior   string        `json:"rec_behavior"`
	BehaviorParas BehaviorParas `json:"behavior_paras"`
}

type QueryResponse struct {
	QueryId      string      `json:"queryId"`
	QueryResult  QueryResult `json:"queryResult"`
	LanguageCode string      `json:"languageCode"`
	Index        int         `json:"index"`
}

// --- End support for QueryResponse override logic ---

type Configuration struct {
	PidFile                 string `json:"pidFile"`
	Livingio_API_Server     string `json:"livingio_api_server"`
	Livingio_API_TTS_Server string `json:"livingio_api_tts_server"`
	Livingio_TTS_Server     string `json:"livingio_tts_server"`
	Livingio_RES_Server     string `json:"livingio_res_server"`
	PostFS                  string `json:"postFS"`
	LogFileName             string `json:"logFileName"`
	EnableDatabaseAndAPI    bool   `json:"enableDatabaseAndAPI"`
	SqliteLocation          string `json:"sqliteLocation"`
}

var (
	conf              Configuration
	useDatabaseAndAPI bool = false
)

func main() {
	log.Println("Starting application...")
	//load config
	confFile := flag.String("c", "emoProxy.conf", "config file to use")
	flag.Parse()

	err := loadConfig(*confFile)
	if err != nil {
		log.Println("can't read conf file", *confFile, "- using default config")
	}
	log.Println("config loaded")
	writePid()

	// disable ssl checks
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	// parse flags
	Port := flag.Int("port", 8080, "http port")
	flag.Parse()
	log.Println("Starting app on port: ", *Port)

	// redirect log
	logFile, err := os.OpenFile(conf.LogFileName, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Panic(err)
	}

	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	useDatabaseAndAPI = conf.EnableDatabaseAndAPI

	if useDatabaseAndAPI {
		log.Println("Database and API enabled")

		dbPath := conf.SqliteLocation
		flagDbPath := flag.String("db", "", "path to the sqlite database file")
		if *flagDbPath != "" {
			dbPath = *flagDbPath
		}
		flag.Parse()
		dbCreateErr := InitDB(dbPath)
		if dbCreateErr != nil {
			log.Panic(dbCreateErr)
		}
	} else {
		log.Println("Note: Database and API disabled")
	}

	// handle time requests
	http.HandleFunc("/time", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		_, dtsDiff := time.Now().Zone()
		resp := emo_time{time.Now().Unix(), dtsDiff} // get offset from tz in query

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	// handle token requests
	http.HandleFunc("/token/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		body := makeApiRequest(r)
		fmt.Fprint(w, body)
	})

	// handle emo requests
	http.HandleFunc("/emo/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		body := makeApiRequest(r)
		fmt.Fprint(w, body)
	})

	// handle home station requests
	http.HandleFunc("/home/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		body := makeApiRequest(r)
		fmt.Fprint(w, body)
	})

	http.HandleFunc("/app/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		resp := emo_code{200, "OK"}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	// handle downloads
	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		body := makeTtsRequest(r)
		fmt.Fprint(w, body)
	})

	// handle tts over api server
	http.HandleFunc("/tts/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		body := makeApiTtsRequest(r)
		fmt.Fprint(w, body)
	})

	//handle res requests (fw update)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)

		body := makeResRequest(r, w)
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, body)
	})

	if useDatabaseAndAPI {
		// proxy-api endpoints
		http.HandleFunc("/proxy-api/requests", func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)

			requests, err := getAllRequests()
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(requests)
		})
		// POST /proxy-api/overrides - add new override
		http.HandleFunc("/proxy-api/override", func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			if r.Method != http.MethodPost {
				http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
				return
			}
			var override map[string]interface{}
			fmt.Println("override body ", r.Body)
			if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
				http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
				return
			}

			if err := saveOverride(override["endpointLookup"].(string), override["payloadLookup"].(string), override["responseLookup"].(string), override["responseOverride"].(string)); err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"status": "override added"})
		})

		http.HandleFunc("/proxy-api/overrides", func(w http.ResponseWriter, r *http.Request) {
			logRequest(r)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)

			requests, err := getAllOverrides()
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(requests)
		})
	}

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(*Port), corsMiddleware(http.DefaultServeMux)))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Replace "*" with "http://localhost:3000" for better security
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")

		// Handle the preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loadConfig(filename string) error {
	DefaultConf := Configuration{
		PidFile:                 "/var/run/emoProxy.pid",
		Livingio_API_Server:     "api.living.ai",
		Livingio_API_TTS_Server: "eu-api.living.ai",
		Livingio_TTS_Server:     "eu-tts.living.ai",
		Livingio_RES_Server:     "res.living.ai",
		PostFS:                  "/tmp/",
		LogFileName:             "/var/log/emoProxy.log",
		EnableDatabaseAndAPI:    false,
		SqliteLocation:          "/var/data/emo_logs.db",
	}

	bytes, err := os.ReadFile(filename)
	if err != nil {
		conf = DefaultConf
		return err
	}

	err = json.Unmarshal(bytes, &DefaultConf)
	if err != nil {
		conf = Configuration{}
		return err
	}

	conf = DefaultConf
	return nil
}

func writePid() {
	if conf.PidFile != "" {
		f, err := os.OpenFile(conf.PidFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatalf("Unable to create pid file : %v", err)
		}
		defer f.Close()

		f.WriteString(fmt.Sprintf("%d", os.Getpid()))
	}
}

func logRequest(r *http.Request) {
	log.Println("request call: ", r)

	for k, v := range r.Header {
		log.Printf("Request-Header field %q, Value %q\n", k, v)
	}
}

func logResponse(r *http.Response) {
	log.Println("responce call: ", r)

	for k, v := range r.Header {
		log.Printf("Response-Header field %q, Value %q\n", k, v)
	}
}

func logBody(contentType string, body []byte, prefix string) {
	// write post request body to fs
	dir := conf.PostFS + time.Now().Format("20060102/")
	os.MkdirAll(dir, os.ModePerm)
	switch contentType {
	case "application/json":
		os.WriteFile(dir+"emo_"+prefix+fmt.Sprint(time.Now().Unix())+".json", body, 0644)
	case "application/octet-stream":
		os.WriteFile(dir+"emo_"+prefix+fmt.Sprint(time.Now().Unix())+".wav", body, 0644)
	case "audio/mpeg":
		os.WriteFile(dir+"emo_"+prefix+fmt.Sprint(time.Now().Unix())+".mp3", body, 0644)
	default:
		os.WriteFile(dir+"emo_"+prefix+fmt.Sprint(time.Now().Unix())+".bin", body, 0644)
	}
}

func makeApiRequest(r *http.Request) string {
	var request *http.Request
	var requestBody []byte
	switch r.Method {
	case "GET":
		request, _ = http.NewRequest("GET", "https://"+conf.Livingio_API_Server+r.URL.RequestURI(), nil)
	case "POST":
		requestBody, _ := io.ReadAll(r.Body)

		// write post request body to fs
		logBody(r.Header.Get("Content-Type"), requestBody, "apiReq_")

		request, _ = http.NewRequest("POST", "https://"+conf.Livingio_API_Server+r.URL.RequestURI(), bytes.NewBuffer(requestBody))

		request.Header.Add("Content-Type", r.Header.Get("Content-Type"))
		request.Header.Add("Content-Length", r.Header.Get("Content-Length"))
	default:
	}

	val, exists := r.Header["Authorization"]
	if exists {
		request.Header.Add("Authorization", val[0])
	}

	val, exists = r.Header["Secret"]
	if exists {
		request.Header.Add("Secret", val[0])
	}

	request.Header.Del("User-Agent")

	httpclient := &http.Client{}
	response, err := httpclient.Do(request)

	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer response.Body.Close()

	// read response
	body, _ := io.ReadAll(response.Body)
	log.Println("Server response: ", string(body))
	fmt.Println("Server response: ", string(body))

	override, success, error := getOverrideBasedOnResponse(string(body))
	fmt.Println("override:", override, " success:", success, " error:", error)

	if success {
		log.Println("Overriding response with: ", override)
		var typedOverride QueryResponse
		if err := json.Unmarshal([]byte(override), &typedOverride); err == nil {
			log.Printf("Typed override: %+v\n", typedOverride)
		}

		var typedBody QueryResponse
		if err := json.Unmarshal([]byte(body), &typedBody); err == nil {
			log.Printf("Typed body: %+v\n", typedBody)
		}

		// typedBody.QueryResult.RecBehavior = typedOverride.QueryResult.RecBehavior
		// typedBody.QueryResult.BehaviorParas = typedOverride.QueryResult.BehaviorParas
		// typedBody.QueryResult.Intent = typedOverride.QueryResult.Intent
		// typedBody.QueryResult = typedOverride.QueryResult

		typedOverride.QueryId = typedBody.QueryId
		typedOverride.QueryResult.ResultCode = typedBody.QueryResult.ResultCode
		typedOverride.Index = typedBody.Index

		fmt.Println("typedBody", typedBody)
		fmt.Println("typedOverride", typedOverride)

		// overrideBytes, err := json.Marshal(typedBody)
		overrideBytes, err := json.Marshal(typedOverride)
		if err != nil {
			log.Fatalf("An Error Occured during marshaling override %v", err)
		}

		fmt.Println("string(overrideBytes)", string(overrideBytes))

		// add so that we log which override is used
		saveRequest(r.URL.RequestURI(), string(requestBody), string(overrideBytes))
		return string(overrideBytes)
	}

	logResponse(response)

	if useDatabaseAndAPI {
		saveRequest(r.URL.RequestURI(), string(requestBody), string(body))
	}
	return string(body)
}

func makeTtsRequest(r *http.Request) string {
	request, _ := http.NewRequest("GET", "http://"+conf.Livingio_TTS_Server+r.URL.RequestURI(), nil)

	val, exists := r.Header["Authorization"]
	if exists {
		request.Header.Add("Authorization", val[0])
	}

	val, exists = r.Header["Secret"]
	if exists {
		request.Header.Add("Secret", val[0])
	}

	request.Header.Del("User-Agent")

	httpclient := &http.Client{}
	response, err := httpclient.Do(request)

	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer response.Body.Close()

	// read response
	body, _ := io.ReadAll(response.Body)

	// write post request body to fs
	logBody(response.Header.Get("Content-Type"), body, "tts_")
	logResponse(response)

	if useDatabaseAndAPI {
		saveRequest(r.URL.RequestURI(), "", "")
	}
	return string(body)
}

func makeApiTtsRequest(r *http.Request) string {
	request, _ := http.NewRequest("GET", "http://"+conf.Livingio_API_TTS_Server+r.URL.RequestURI(), nil)

	val, exists := r.Header["Authorization"]
	if exists {
		request.Header.Add("Authorization", val[0])
	}

	val, exists = r.Header["Secret"]
	if exists {
		request.Header.Add("Secret", val[0])
	}

	request.Header.Del("User-Agent")

	httpclient := &http.Client{}
	response, err := httpclient.Do(request)

	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer response.Body.Close()

	// read response
	body, _ := io.ReadAll(response.Body)

	// write post request body to fs
	logBody(response.Header.Get("Content-Type"), body, "apitts_")
	logResponse(response)

	if useDatabaseAndAPI {
		saveRequest(r.URL.RequestURI(), "", string(body))
	}
	return string(body)
}

func makeResRequest(r *http.Request, w http.ResponseWriter) string {
	request, _ := http.NewRequest("GET", "https://"+conf.Livingio_RES_Server+r.URL.RequestURI(), nil)

	val, exists := r.Header["Authorization"]
	if exists {
		request.Header.Add("Authorization", val[0])
	}

	val, exists = r.Header["Secret"]
	if exists {
		request.Header.Add("Secret", val[0])
	}

	request.Header.Del("User-Agent")

	httpclient := &http.Client{}
	response, err := httpclient.Do(request)

	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer response.Body.Close()

	// read response
	body, _ := io.ReadAll(response.Body)

	logBody(response.Header.Get("Content-Type"), body, "res_")

	for k := range response.Header {
		w.Header().Set(k, response.Header.Get(k))
	}

	logResponse(response)

	if useDatabaseAndAPI {
		saveRequest(r.URL.RequestURI(), "", string(body))
	}
	return string(body)
}
