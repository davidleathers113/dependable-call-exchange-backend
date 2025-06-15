//go:build security

package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/auth"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Removed duplicate TestSecurity_Authentication - keeping the one in auth_security_complete_test.go

// Removed duplicate helper functions - they exist in token_helpers_test.go
