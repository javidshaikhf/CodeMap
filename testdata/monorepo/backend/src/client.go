package src

import "net/http"

type APIClient struct{}

func NewClient() *APIClient {
	return &APIClient{}
}
