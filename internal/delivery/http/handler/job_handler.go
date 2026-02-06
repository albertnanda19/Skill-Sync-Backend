package handler

import "net/http"

type JobHandler struct{}

func NewJobHandler() *JobHandler {
	return &JobHandler{}
}

func (h *JobHandler) RegisterRoutes(_ interface{}) {}

func (h *JobHandler) Handler() http.Handler {
	return nil
}
