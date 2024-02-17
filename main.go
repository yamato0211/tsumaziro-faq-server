package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/cors"

	"firebase.google.com/go/auth"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"github.com/yamato0211/tsumaziro-faq-server/batch"
	"github.com/yamato0211/tsumaziro-faq-server/db/model"
	cfg "github.com/yamato0211/tsumaziro-faq-server/pkg/config"
	connector "github.com/yamato0211/tsumaziro-faq-server/pkg/db"
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

type GetTitleRequest struct {
	PageTitle string `json:"page_title"`
}

type ScrapBoxResponse struct {
	Descriptions []string `json:"descriptions"`
}

func NewAuthMiddelware(fc *auth.Client) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idToken := r.Header.Get("Authorization")
			if idToken == "" {
				log.Println("empty idToken")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			token, err := fc.VerifyIDToken(r.Context(), idToken)
			if err != nil {
				log.Println("error verifying ID token: ", err)
				http.Error(w, "Unauthorized Error: verifying ID token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), "user_id", token.UID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func NewSubDomainMiddelware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := os.Getenv("API_HOST")
			if host == r.Host {
				next.ServeHTTP(w, r)
				return
			}
			splits := strings.Split(r.Host, ".")
			subDomain := splits[0]
			log.Println("subDomain: ", subDomain)
			ctx := context.WithValue(r.Context(), "sub_domain", subDomain)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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

	dbCfg := cfg.NewDBConfig()
	db := connector.NewMySQLConnector(dbCfg)

	// fbCfg := cfg.NewFirebaseConfig()
	// fc, err := firebase.NewFirebaseAuthApp(context.Background(), fbCfg)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	subDomainMiddleware := NewSubDomainMiddelware()

	ticker := time.NewTicker(5 * time.Minute)
	done := make(chan bool)

	defer func() {
		done <- true
	}()

	go func(db *connector.DB) {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := batch.BatchGenerateFAQ(db, context.Background()); err != nil {
					log.Println("Error: ", err)
				}
			}
		}
	}(db)

	faqHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subDomain, ok := r.Context().Value("sub_domain").(string)
		if !ok {
			log.Println("Internal server error: sub_domain type is not string")
			http.Error(w, "Internal server error: sub_domain type is not string", http.StatusInternalServerError)
			return
		}
		if subDomain == "" {
			log.Println("Not Found Sub Domain")
			http.Error(w, "Not Found Sub Domain", http.StatusNotFound)
			return
		}
		var account *model.Account
		if err := db.DB.NewSelect().Model((*model.Account)(nil)).Where("id = ?", subDomain).Scan(r.Context(), &account); err != nil {
			log.Println("Not Found Sub Domain User: ", err)
			http.Error(w, "Not Found Sub Domain User: "+err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write(account.Faqs); err != nil {
			log.Println("Internal server error: ", err)
			http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})

	getTitleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subDomain, ok := r.Context().Value("sub_domain").(string)
		if !ok {
			log.Println("Internal server error: sub_domain type is not string")
			http.Error(w, "Internal server error: sub_domain type is not string", http.StatusInternalServerError)
			return
		}
		if subDomain == "" {
			log.Println("Not Found Sub Domain")
			http.Error(w, "Not Found Sub Domain", http.StatusNotFound)
			return
		}
		var account *model.Account
		if err := db.DB.NewSelect().Model((*model.Account)(nil)).Where("id = ?", subDomain).Scan(r.Context(), &account); err != nil {
			log.Println("Not Found Sub Domain User: ", err)
			http.Error(w, "Not Found Sub Domain User: "+err.Error(), http.StatusNotFound)
			return
		}
		var req GetTitleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Println("Status Bad Request: ", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := http.Get("https://scrapbox.io/api/pages/" + subDomain + "/" + req.PageTitle)
		if err != nil {
			log.Println("Internal server error: ", err)
			http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("Internal Server Error: ", err)
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err = w.Write(body); err != nil {
			log.Println("Internal server error: ", err)
			http.Error(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	})

	bedrockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			log.Println("Couldn't marshal the request: ", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
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

	mux.HandleFunc("GET /faq", subDomainMiddleware(faqHandler))

	mux.HandleFunc("POST /faq", subDomainMiddleware(getTitleHandler))

	mux.HandleFunc("POST /bedrock", subDomainMiddleware((bedrockHandler)))

	mux.HandleFunc("GET /", subDomainMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, world!")
	})))

	c := cors.Default()
	corsMux := c.Handler(mux)

	log.Println("listen and serve ... on port 8080")
	if err := http.ListenAndServe(":8080", corsMux); err != nil {
		log.Fatal(err)
	}
}
