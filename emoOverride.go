package main

import (
	"encoding/json"
	"fmt"
	"log"
)

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

func overrideApiRequest(body []byte) string {
	fmt.Println("Server response: ", string(body)) // TODO: remove later

	override, success, error := getOverrideBasedOnResponse(string(body))
	fmt.Println("override:", override, " success:", success, " error:", error) // TODO: remove later or change to log

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

		fmt.Println("typedBody", typedBody)         // TODO: remove later
		fmt.Println("typedOverride", typedOverride) // TODO: remove later

		// overrideBytes, err := json.Marshal(typedBody)
		overrideBytes, err := json.Marshal(typedOverride)
		if err != nil {
			log.Fatalf("An Error Occured during marshaling override %v", err)
		}

		fmt.Println("string(overrideBytes)", string(overrideBytes)) // TODO: remove later

		// add so that we log which override is used
		return string(overrideBytes)
	}

	return ""
}
