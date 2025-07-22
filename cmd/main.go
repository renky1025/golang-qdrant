package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"goqdrantapp/qdrantUtils"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

const (
	qdrantAddress = "10.100.2.1:6334"
	collection    = "test_collection"
	vectorSize    = 1536
)

type addRequest struct {
	Text    string            `json:"text"`
	Vector  []float32         `json:"vector"`
	Payload map[string]string `json:"payload"`
}

type deleteRequest struct {
	Collection string `json:"collection"`
}

type embeddingMode string

const (
	embeddingOpenAI embeddingMode = "openai"
	embeddingOllama embeddingMode = "ollama"
	embeddingLocal  embeddingMode = "local"
)

// 配置：通过环境变量或常量切换 embedding 源
var (
	embeddingSource = embeddingOpenAI // 可选：embeddingOpenAI, embeddingOllama, embeddingLocal
	openaiKey       = os.Getenv("OPENAI_API_KEY")
	ollamaURL       = "http://localhost:11434/api/embeddings"
	localEmbedURL   = "http://localhost:8000/embed" // 假设本地embedding服务
)

var openaiClient *openai.Client

func main() {
	if os.Getenv("EMBEDDING_MODE") != "" {
		embeddingSource = embeddingMode(os.Getenv("EMBEDDING_MODE"))
	}
	if openaiKey == "" && embeddingSource == embeddingOpenAI {
		log.Fatal("请设置 OPENAI_API_KEY 环境变量")
	}
	if embeddingSource == embeddingOpenAI {
		openaiClient = openai.NewClient(openaiKey)
	}

	client := qdrantUtils.NewQdrantClient(qdrantAddress, collection, vectorSize)
	defer client.Close()

	// 搜索接口
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		searchTerm := r.URL.Query().Get("term")
		if searchTerm == "" {
			http.Error(w, "Missing search term", http.StatusBadRequest)
			return
		}
		vector, err := getEmbedding(searchTerm)
		if err != nil {
			http.Error(w, "Embedding error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if len(vector) != vectorSize {
			http.Error(w, "Embedding size mismatch", http.StatusInternalServerError)
			return
		}
		results, err := client.Search(vector)
		if err != nil {
			http.Error(w, "Qdrant search error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(results)
	})

	// 添加向量接口
	http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req addRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		var vector []float32
		if len(req.Vector) == vectorSize {
			vector = req.Vector
		} else if req.Text != "" {
			v, err := getEmbedding(req.Text)
			if err != nil {
				http.Error(w, "Embedding error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			vector = v
		} else {
			http.Error(w, "Missing vector or text", http.StatusBadRequest)
			return
		}
		if len(vector) != vectorSize {
			http.Error(w, "Embedding size mismatch", http.StatusInternalServerError)
			return
		}
		id := uuid.New().String()
		err := client.CreatePoint(id, collection, vector, req.Payload)
		if err != nil {
			http.Error(w, "Failed to add vector: "+err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "ok"})
	})

	// 删除集合接口
	http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req deleteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Collection == "" {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		err := client.DeleteCollection(req.Collection)
		if err != nil {
			http.Error(w, "Failed to delete collection: "+err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Println("Starting HTTP server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// getEmbedding 根据配置选择 embedding 源
func getEmbedding(text string) ([]float32, error) {
	switch embeddingSource {
	case embeddingOpenAI:
		return getOpenAIEmbedding(text)
	case embeddingOllama:
		return getOllamaEmbedding(text)
	case embeddingLocal:
		return getLocalEmbedding(text)
	default:
		return nil, nil
	}
}

// getOpenAIEmbedding 调用 OpenAI embedding
func getOpenAIEmbedding(text string) ([]float32, error) {
	resp, err := openaiClient.CreateEmbeddings(
		context.Background(),
		openai.EmbeddingRequest{
			Input: []string{text},
			Model: openai.AdaEmbeddingV2,
		},
	)
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	// float64 -> float32
	vec := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		vec[i] = float32(v)
	}
	return vec, nil
}

// getOllamaEmbedding 调用 Ollama embedding API
func getOllamaEmbedding(text string) ([]float32, error) {
	body := map[string]interface{}{
		"model":  "nomic-embed-text", // 可根据实际 ollama 模型名调整
		"prompt": text,
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(ollamaURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}

// getLocalEmbedding 调用本地 embedding HTTP 服务
func getLocalEmbedding(text string) ([]float32, error) {
	body := map[string]string{"text": text}
	b, _ := json.Marshal(body)
	resp, err := http.Post(localEmbedURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	var result struct {
		Vector []float32 `json:"vector"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Vector, nil
}
