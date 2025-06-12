package rest

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/analytics"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
)

// GoldStandardServices holds all service dependencies for the 11/10 API
type GoldStandardServices struct {
	Analytics    analytics.Service
	Bidding      bidding.Service
	CallRouting  callrouting.Service
	Fraud        fraud.Service
	Telephony    telephony.Service
}

// HandlersWithServices creates handlers with real service implementations
type HandlersWithServices struct {
	*HandlersV2
	services *GoldStandardServices
}

// NewHandlersWithServices creates handlers connected to real services
func NewHandlersWithServices(services *GoldStandardServices, config *Config) *HandlersWithServices {
	if config == nil {
		config = DefaultConfig()
	}

	return &HandlersWithServices{
		HandlersV2: NewHandlersV2(config.Version, config.BaseURL),
		services:   services,
	}
}

// CreateGoldStandardAPI creates the complete 11/10 API setup
func CreateGoldStandardAPI(services *GoldStandardServices) http.Handler {
	// Create configuration
	config := &Config{
		Version:               "v1",
		BaseURL:               getEnvOrDefault("API_BASE_URL", "https://api.dependablecallexchange.com"),
		EnableMetrics:         true,
		EnableTracing:         true,
		EnableCompression:     true,
		CompressionMinSize:    1024,
		EnableRateLimiting:    true,
		PublicRateLimit:       10,
		PublicRateBurst:       20,
		AuthRateLimit:         100,
		AuthRateBurst:         200,
		EnableCircuitBreaker:  true,
		CircuitBreakerTimeout: 30 * time.Second,
		CacheDuration:         5 * time.Minute,
		EnableWebSocket:       true,
		EnableGraphQL:         false, // Coming soon
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	}

	// Create router with all the gold standard features
	return NewRouter(config)
}

// MigrateFromBasicHandlers provides a migration path from the basic handlers
func MigrateFromBasicHandlers(h *Handler) http.Handler {
	// Extract services from the old handlers and create enhanced services
	services := &GoldStandardServices{
		Bidding:     h.Services.Bidding,
		CallRouting: h.Services.CallRouting,
		Fraud:       h.Services.Fraud,
		Telephony:   h.Services.Telephony,
		// Analytics would need to be injected separately as it's not in the old Services
		Analytics:   nil, // TODO: Inject analytics service
	}

	// Create the gold standard API
	return CreateGoldStandardAPI(services)
}

// Example of how to use in main.go:
//
// func main() {
//     // Initialize your services
//     services := &rest.GoldStandardServices{
//         Analytics:   analyticsService,
//         Bidding:     biddingService,
//         CallRouting: callRoutingService,
//         Fraud:       fraudService,
//         Marketplace: marketplaceService,
//         Telephony:   telephonyService,
//     }
//
//     // Create the gold standard API
//     api := rest.CreateGoldStandardAPI(services)
//
//     // Start the server
//     server := &http.Server{
//         Addr:    ":8080",
//         Handler: api,
//     }
//
//     log.Fatal(server.ListenAndServe())
// }

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}