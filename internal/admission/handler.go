package admission

import (
	"encoding/json"
	"net/http"
)

type Handler struct{}

func (Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var review Review
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, "invalid AdmissionReview", http.StatusBadRequest)
		return
	}
	result := Validate(review.Request)
	uid := ""
	if review.Request != nil {
		uid = review.Request.UID
	}
	response := Review{
		APIVersion: "admission.k8s.io/v1",
		Kind:       "AdmissionReview",
		Response:   &Response{UID: uid, Allowed: result.Allowed},
	}
	if !result.Allowed {
		response.Response.Status = &Status{Code: 403, Reason: "Forbidden", Message: result.Message}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
