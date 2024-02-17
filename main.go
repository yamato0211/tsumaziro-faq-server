package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type ClaudeRequest struct {
	Prompt            string `json:"prompt"`
	MaxTokensToSample int    `json:"max_tokens_to_sample"`
	// Omitting optional request parameters
}

type ClaudeResponse struct {
	Completion string `json:"completion"`
}

type BedrockRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type BedrockResponse struct {
	Completion string `json:"completion"`
}

func main() {
	region := flag.String("region", "us-east-1", "The AWS region")
	flag.Parse()

	fmt.Printf("Using AWS region: %s\n", *region)

	sdkConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(*region))
	if err != nil {
		fmt.Println("Couldn't load default configuration. Have you set up your AWS account?")
		fmt.Println(err)
		return
	}

	client := bedrockruntime.NewFromConfig(sdkConfig)
	modelId := "anthropic.claude-v2"
	prefix := "Human: "
	postfix := "\n\nAssistant:"

	mux := http.NewServeMux()

	mux.HandleFunc("POST /hello", func(w http.ResponseWriter, r *http.Request) {
		req := BedrockRequest{Model: modelId}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		wrappedPrompt := prefix + req.Prompt + postfix

		request := ClaudeRequest{
			Prompt:            wrappedPrompt,
			MaxTokensToSample: 200,
		}

		body, err := json.Marshal(request)
		if err != nil {
			log.Panicln("Couldn't marshal the request: ", err)
		}

		result, err := client.InvokeModel(context.Background(), &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(req.Model),
			ContentType: aws.String("application/json"),
			Body:        body,
		})

		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "no such host") {
				fmt.Printf("Error: The Bedrock service is not available in the selected region. Please double-check the service availability for your region at https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/.\n")
			} else if strings.Contains(errMsg, "Could not resolve the foundation model") {
				fmt.Printf("Error: Could not resolve the foundation model from model identifier: \"%v\". Please verify that the requested model exists and is accessible within the specified region.\n", modelId)
			} else {
				fmt.Printf("Error: Couldn't invoke Anthropic Claude. Here's why: %v\n", err)
			}
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var response ClaudeResponse

		err = json.Unmarshal(result.Body, &response)

		if err != nil {
			log.Fatal("failed to unmarshal", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		res := BedrockResponse{
			Completion: response.Completion,
		}
		json.NewEncoder(w).Encode(res)
	})

	log.Println("listen and serve ... on port 8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
