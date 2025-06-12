package rest

import (
	"net/http"
)

// handleDocs serves the API documentation
func (h *Handler) handleDocs(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement swagger UI or other documentation
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>Dependable Call Exchange API Documentation</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        .endpoint { margin: 20px 0; padding: 10px; background: #f5f5f5; }
        .method { font-weight: bold; color: #2196F3; }
        .path { color: #4CAF50; }
    </style>
</head>
<body>
    <h1>Dependable Call Exchange API v1</h1>
    <p>Welcome to the API documentation. Full OpenAPI specification coming soon.</p>
    
    <h2>Available Endpoints</h2>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/health</span> - Health check
    </div>
    
    <div class="endpoint">
        <span class="method">POST</span> <span class="path">/api/v1/calls</span> - Create a new call
    </div>
    
    <div class="endpoint">
        <span class="method">POST</span> <span class="path">/api/v1/bids</span> - Place a bid
    </div>
    
    <div class="endpoint">
        <span class="method">POST</span> <span class="path">/api/v1/auctions</span> - Create an auction
    </div>
    
    <p>For the full OpenAPI specification, visit <a href="/docs/openapi.json">/docs/openapi.json</a></p>
</body>
</html>
    `))
}

// handleOpenAPISpec serves the OpenAPI specification
func (h *Handler) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	// TODO: Generate OpenAPI spec using reflection or code generation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{
  "openapi": "3.0.3",
  "info": {
    "title": "Dependable Call Exchange API",
    "description": "Real-time call routing and bidding marketplace",
    "version": "1.0.0",
    "contact": {
      "name": "API Support",
      "email": "support@dependablecallexchange.com"
    }
  },
  "servers": [
    {
      "url": "/api/v1",
      "description": "API v1"
    }
  ],
  "paths": {
    "/calls": {
      "post": {
        "summary": "Create a new call",
        "operationId": "createCall",
        "tags": ["Calls"],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateCallRequest"
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Call created successfully",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Call"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "CreateCallRequest": {
        "type": "object",
        "required": ["from_number", "to_number"],
        "properties": {
          "from_number": {
            "type": "string",
            "description": "Caller's phone number"
          },
          "to_number": {
            "type": "string",
            "description": "Recipient's phone number"
          },
          "direction": {
            "type": "string",
            "enum": ["inbound", "outbound"],
            "default": "outbound"
          }
        }
      },
      "Call": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "format": "uuid"
          },
          "from_number": {
            "type": "string"
          },
          "to_number": {
            "type": "string"
          },
          "status": {
            "type": "string"
          },
          "created_at": {
            "type": "string",
            "format": "date-time"
          }
        }
      }
    },
    "securitySchemes": {
      "bearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      }
    }
  },
  "security": [
    {
      "bearerAuth": []
    }
  ]
}`))
}

// handleWebSocket handles WebSocket connections
func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket handling is implemented in websocket_handler.go
	// This is just a placeholder to satisfy the route registration
	h.writeError(w, http.StatusNotImplemented, "WEBSOCKET_NOT_IMPLEMENTED", "WebSocket support coming soon", "")
}