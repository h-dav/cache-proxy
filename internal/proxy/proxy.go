package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"sync"
	"time"
)

type Cache struct {
	Response     *http.Response
	ResponseBody []byte
	Created      time.Time
}

// FlushCache will clear all data from the cache.
func (p *Proxy) FlushCache() {
	p.Mutex.Lock()
	p.Cache = make(map[string]*Cache)
	p.Mutex.Unlock()
}

// CleanCache clears the cache for a specific tag.
func (p *Proxy) CleanCache(cacheTag string) {
	p.Cache[cacheTag] = &Cache{}
}

type Proxy struct {
	HashKey hash.Hash
	Origin  string
	Cache   map[string]*Cache
	Mutex   sync.Mutex
}

func New(origin string) *Proxy {
	return &Proxy{
		HashKey: sha256.New(),
		Origin:  origin,
		Cache:   make(map[string]*Cache),
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			http.Error(w, "Error Forwarding Request", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Error Forwarding Request Body", http.StatusInternalServerError)
			return
		}

		respondWithHeaders(w, *resp, responseBody, "miss", "")
		return
	}

	cacheTag, err := p.determineCacheTag(r)
	if err != nil {
		http.Error(w, "Error determining cache key", http.StatusInternalServerError)
		return
	}

	p.Mutex.Lock()
	if c, ok := p.Cache[cacheTag]; ok {
		p.Mutex.Unlock()
		respondWithHeaders(w, *c.Response, c.ResponseBody, "hit", cacheTag)
		return
	}
	p.Mutex.Unlock()

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		http.Error(w, "Error Forwarding Request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error Forwarding Request Body", http.StatusInternalServerError)
		return
	}

	p.Mutex.Lock()
	p.Cache[cacheTag] = &Cache{
		Response:     resp,
		ResponseBody: responseBody,
		Created:      time.Now(),
	}
	p.Mutex.Unlock()

	respondWithHeaders(w, *resp, responseBody, "miss", cacheTag)
}

func (p *Proxy) determineCacheTag(r *http.Request) (string, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	b := string(body)

	plainTag := r.Method + ":" + r.URL.String() + ":" + b

	p.HashKey.Write([]byte(plainTag))

	hashSum := p.HashKey.Sum(nil)

	cacheTag := hex.EncodeToString(hashSum)

	return cacheTag, nil
}

func respondWithHeaders(w http.ResponseWriter, response http.Response, body []byte, cacheHeader, key string) {
	w.Header().Set("X-Cache", cacheHeader)

	w.WriteHeader(response.StatusCode)

	for k, v := range response.Header {
		w.Header()[k] = v
	}

	w.Write(body)
}
