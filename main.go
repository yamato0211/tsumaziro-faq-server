package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/cors"

	"firebase.google.com/go/auth"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/yamato0211/tsumaziro-faq-server/batch"
	"github.com/yamato0211/tsumaziro-faq-server/db/model"
	cfg "github.com/yamato0211/tsumaziro-faq-server/pkg/config"
	connector "github.com/yamato0211/tsumaziro-faq-server/pkg/db"
	"github.com/yamato0211/tsumaziro-faq-server/pkg/firebase"
)

const (
	BucketName   = "ottottottotto"
	htmlFileName = "index.html"
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

type CreateAccountRequest struct {
	SubDomain  string `json:"sub_domain"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	FirebaseID string `json:"firebase_id"`
	ProjectID  string `json:"project_id"`
	URL        string `json:"url"`
}

type CrawlData struct {
	SubDomain string `json:"sub_domain"`
	URL       string `json:"url"`
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
			// log.Println("subDomain: ", subDomain)
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

	client := bedrockagentruntime.NewFromConfig(sdkConfig)
	s3Client := s3.NewFromConfig(sdkConfig)

	mux := http.NewServeMux()

	dbCfg := cfg.NewDBConfig()
	db := connector.NewMySQLConnector(dbCfg)

	fbCfg := cfg.NewFirebaseConfig()
	fc, err := firebase.NewFirebaseAuthApp(context.Background(), fbCfg)
	if err != nil {
		log.Fatal(err)
	}

	subDomainMiddleware := NewSubDomainMiddelware()

	ticker := time.NewTicker(5 * time.Minute)
	done := make(chan bool)
	crawlData := make(chan CrawlData)

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
			case data := <-crawlData:
				objectKey := data.SubDomain + "/" + htmlFileName
				if err := batch.CrawlKnowledge(data.URL, BucketName, objectKey, s3Client); err != nil {
					log.Println("Error: ", err)
				}
			}
		}
	}(db)

	faqHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// subDomain, ok := r.Context().Value("sub_domain").(string)
		subDomain := r.PathValue("id")
		log.Println("subDomain: ", subDomain)
		// if !ok {
		// 	log.Println("Internal server error: sub_domain type is not string")
		// 	http.Error(w, "Internal server error: sub_domain type is not string", http.StatusInternalServerError)
		// 	return
		// }
		if subDomain == "" {
			log.Println("Not Found Sub Domain")
			http.Error(w, "Not Found Sub Domain", http.StatusNotFound)
			return
		}
		var account model.Account
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
		// subDomain, ok := r.Context().Value("sub_domain").(string)
		subDomain := r.PathValue("id")
		// if !ok {
		// 	log.Println("Internal server error: sub_domain type is not string")
		// 	http.Error(w, "Internal server error: sub_domain type is not string", http.StatusInternalServerError)
		// 	return
		// }
		if subDomain == "" {
			log.Println("Not Found Sub Domain")
			http.Error(w, "Not Found Sub Domain", http.StatusNotFound)
			return
		}
		var account model.Account
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

	createAccountHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CreateAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		account := &model.Account{
			ID:         req.SubDomain,
			Name:       req.Name,
			Email:      req.Email,
			ProjectID:  req.ProjectID,
			FirebaseID: req.FirebaseID,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if _, err := db.DB.NewInsert().Model(account).Exec(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		crawlData <- CrawlData{
			SubDomain: req.SubDomain,
			URL:       req.URL,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
	})

	bedrockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := BedrockRequest{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		prompt := req.Prompt

		userID := fmt.Sprintf("%d", rand.Intn(1000000))

		output, err := client.InvokeAgent(context.Background(), &bedrockagentruntime.InvokeAgentInput{
			InputText:    aws.String(prompt),
			AgentId:      aws.String("W5PUPQIIS8"),
			AgentAliasId: aws.String("GLYSWGXVOT"),
			SessionId:    aws.String(userID),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		text := ""
		for event := range output.GetStream().Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				text += string(v.Value.Bytes)

			case *types.UnknownUnionMember:
				fmt.Println("unknown tag:", v.Tag)

			default:
				fmt.Println("union is nil or unknown type")
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		res := BedrockResponse{
			Completion: text,
		}
		json.NewEncoder(w).Encode(res)
	})

	mux.HandleFunc("GET /{id}/faq", subDomainMiddleware(faqHandler))

	mux.HandleFunc("POST /{id}/faq", subDomainMiddleware(getTitleHandler))

	mux.HandleFunc("POST /account", NewAuthMiddelware(fc)(createAccountHandler))

	mux.HandleFunc("POST /{id}/bedrock", subDomainMiddleware((bedrockHandler)))

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
