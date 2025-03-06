package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	constants "github.com/kubetracer/kubetracer-go/constants"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// WebhookServer handles admission requests
type WebhookServer struct {
	decoder *admission.Decoder
}

// Handle handles the admission review requests
func (a *WebhookServer) Handle(w http.ResponseWriter, r *http.Request) {
	admissionReview := admissionv1.AdmissionReview{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not read request body: %v", err), http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not decode admission review: %v", err), http.StatusBadRequest)
		return
	}

	req := admissionReview.Request
	user := req.UserInfo.Username
	userID := os.Getenv("USER_ID")

	if user != userID {
		// Check if the annotation exists and create the JSON patch to remove it
		var resource map[string]interface{}
		if err := json.Unmarshal(req.Object.Raw, &resource); err != nil {
			http.Error(w, fmt.Sprintf("could not unmarshal object: %v", err), http.StatusBadRequest)
			return
		}

		// Navigate the map to find and remove the annotation if it exists
		if metadata, found := resource["metadata"].(map[string]interface{}); found {
			if annotations, found := metadata["annotations"].(map[string]interface{}); found {
				if _, exists := annotations[constants.TraceIDAnnotation]; exists {
					patch := []map[string]string{{
						"op":   "remove",
						"path": "/metadata/annotations/" + constants.TraceIDAnnotation,
					}}
					patchBytes, err := json.Marshal(patch)
					if err != nil {
						http.Error(w, fmt.Sprintf("could not marshal patch: %v", err), http.StatusInternalServerError)
						return
					}

					response := admissionv1.AdmissionResponse{
						UID:       req.UID,
						Allowed:   true,
						Patch:     patchBytes,
						PatchType: func() *admissionv1.PatchType { pt := admissionv1.PatchTypeJSONPatch; return &pt }(),
					}
					admissionReview.Response = &response
					respBytes, err := json.Marshal(admissionReview)
					if err != nil {
						http.Error(w, fmt.Sprintf("could not marshal response: %v", err), http.StatusInternalServerError)
						return
					}
					w.WriteHeader(http.StatusOK)
					w.Write(respBytes)
					return
				}
			}
		}
	}

	// If the annotation does not exist or the user matches USER_ID, allow the request without modification
	response := admissionv1.AdmissionResponse{
		UID:     req.UID,
		Allowed: true,
	}
	admissionReview.Response = &response
	respBytes, err := json.Marshal(admissionReview)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not marshal response: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)
}

func main() {
	userID := os.Getenv("USER_ID")
	if userID == "" {
		log.Fatalf("USER_ID env variable is not set")
	}

	mux := http.NewServeMux()
	webhookServer := &WebhookServer{}
	mux.HandleFunc("/mutate", webhookServer.Handle)

	server := &http.Server{
		Addr:    ":443",
		Handler: mux,
	}

	log.Println("Starting webhook server...")
	if err := server.ListenAndServeTLS("/certs/tls.crt", "/certs/tls.key"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
