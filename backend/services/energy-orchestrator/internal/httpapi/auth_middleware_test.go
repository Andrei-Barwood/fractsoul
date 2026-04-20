package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuthMiddlewareAttachesPrincipalScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(APIKeyAuthMiddleware(nil, APIKeyAuthConfig{
		Enabled:         true,
		Header:          "X-API-Key",
		Keys:            []string{"viewer-key"},
		RBACEnabled:     true,
		DefaultRole:     RoleViewer,
		KeyRoles:        map[string]string{"viewer-key": RoleViewer},
		KeyPrincipalIDs: map[string]string{"viewer-key": "ops.viewer"},
		KeySiteScopes:   map[string][]string{"viewer-key": []string{"site-cl-01"}},
		KeyRackScopes:   map[string][]string{"viewer-key": []string{"rack-cl-01-01"}},
	}))
	router.GET("/principal", func(c *gin.Context) {
		c.JSON(http.StatusOK, Principal(c))
	})

	req := httptest.NewRequest(http.MethodGet, "/principal", nil)
	req.Header.Set("X-API-Key", "viewer-key")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}

	var principal AuthPrincipal
	if err := json.Unmarshal(recorder.Body.Bytes(), &principal); err != nil {
		t.Fatalf("unmarshal principal response: %v", err)
	}
	if principal.PrincipalID != "ops.viewer" {
		t.Fatalf("expected principal id ops.viewer, got %q", principal.PrincipalID)
	}
	if principal.Role != RoleViewer {
		t.Fatalf("expected viewer role, got %q", principal.Role)
	}
	if len(principal.AllowedSiteIDs) != 1 || principal.AllowedSiteIDs[0] != "site-cl-01" {
		t.Fatalf("expected one allowed site scope, got %#v", principal.AllowedSiteIDs)
	}
}

func TestRequireSiteAccessRejectsUnauthorizedSite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(APIKeyAuthMiddleware(nil, APIKeyAuthConfig{
		Enabled:       true,
		Header:        "X-API-Key",
		Keys:          []string{"operator-key"},
		RBACEnabled:   true,
		DefaultRole:   RoleOperator,
		KeyRoles:      map[string]string{"operator-key": RoleOperator},
		KeySiteScopes: map[string][]string{"operator-key": []string{"site-cl-01"}},
	}))

	siteGroup := router.Group("/sites/:site_id")
	siteGroup.Use(RequireSiteAccess(nil))
	siteGroup.GET("/budget", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/sites/site-cl-02/budget", nil)
	req.Header.Set("X-API-Key", "operator-key")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 status, got %d", recorder.Code)
	}
}

func TestEnsureRackAccessRejectsUnauthorizedRack(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(APIKeyAuthMiddleware(nil, APIKeyAuthConfig{
		Enabled:       true,
		Header:        "X-API-Key",
		Keys:          []string{"operator-key"},
		RBACEnabled:   true,
		DefaultRole:   RoleOperator,
		KeyRoles:      map[string]string{"operator-key": RoleOperator},
		KeySiteScopes: map[string][]string{"operator-key": []string{"site-cl-01"}},
		KeyRackScopes: map[string][]string{"operator-key": []string{"rack-cl-01-01"}},
	}))

	router.POST("/sites/:site_id/recommendations/reviews", func(c *gin.Context) {
		if !EnsureRackAccess(c, []string{"rack-cl-01-02"}) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/sites/site-cl-01/recommendations/reviews", nil)
	req.Header.Set("X-API-Key", "operator-key")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 status, got %d", recorder.Code)
	}
}
