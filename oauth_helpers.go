package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// OAuth timing constants.
const (
	oauthStateMaxAge = 600 // seconds; 10 minutes is plenty for the round trip
	oauthStateBytes  = 16
	oauthHTTPTimeout = 30 * time.Second
)

// renderOAuthError writes a simple error page. Kept minimal (no new template)
// since OAuth errors are rare and a plain HTML response is fine.
func renderOAuthError(c *gin.Context, msg string) {
	html := oauthErrorHTML(msg)
	c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(html))
	c.Abort()
}

func oauthErrorHTML(msg string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="light">
<head><meta charset="utf-8"><title>Sign-in error</title>
<link rel="stylesheet" href="/static/styles.css"></head>
<body class="min-h-screen bg-base-200 grid place-content-center p-8">
<div class="card bg-base-100 shadow-lg max-w-md">
<div class="card-body">
<h1 class="text-xl font-semibold mb-2">Sign-in error</h1>
<p class="text-base-content/70 mb-4">%s</p>
<a href="/auth/login" class="btn btn-primary">Try again</a>
</div></div></body></html>`, msg)
}
