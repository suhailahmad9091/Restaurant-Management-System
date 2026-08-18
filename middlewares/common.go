package middlewares

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

func corsOptions() *cors.Cors {
	return cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},                                                                                                                                                                 // Allow all origins, adjust as necessary for production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},                                                                                                                  // HTTP methods allowed by CORS
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Access-Token", "importDate", "X-Client-Version", "Cache-Control", "Pragma", "x-started-at", "x-api-key"}, // Allowed headers for CORS
		ExposedHeaders:   []string{"Link"},                                                                                                                                                              // Headers that are exposed to the client
		AllowCredentials: true,                                                                                                                                                                          // Allow credentials such as cookies or authorization headers
	})
}

func CommonMiddlewares() chi.Middlewares {
	return chi.Chain(
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Content-Type", "application/json")
				next.ServeHTTP(w, r)
			})
		},
		corsOptions().Handler,
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() {
					err := recover()
					if err != nil {
						logrus.Errorf("Request Panic err: %v", err)
						jsonBody, _ := json.Marshal(map[string]string{
							"error": "There was an internal server error",
						})
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)
						_, err := w.Write(jsonBody)
						if err != nil {
							logrus.Errorf("Failed to send response from middleware with error: %+v", err) // Log error if response fails to send
						}
					}
				}()
				next.ServeHTTP(w, r)
			})
		},
	)
}
