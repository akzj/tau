package server

import "github.com/gin-gonic/gin"

func (s *Server) handleOpenAPI(c *gin.Context) {
	spec := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "tau API",
			"version":     "0.1.0",
			"description": "tau agent API server — chat, sessions, tools",
		},
		"servers": []map[string]any{
			{"url": "http://" + s.host + ":" + s.port},
		},
		"paths": map[string]any{
			"/v1/health": map[string]any{
				"get": map[string]any{
					"summary":     "Health check",
					"operationId": "health",
					"responses":   map[string]any{"200": map[string]any{"description": "OK"}},
				},
			},
			"/v1/tools": map[string]any{
				"get": map[string]any{
					"summary":     "List available tools",
					"operationId": "listTools",
					"responses":   map[string]any{"200": map[string]any{"description": "Tool list"}},
				},
			},
			"/v1/sessions": map[string]any{
				"get": map[string]any{
					"summary":     "List sessions",
					"operationId": "listSessions",
					"responses":   map[string]any{"200": map[string]any{"description": "Session list"}},
				},
				"post": map[string]any{
					"summary":     "Create session",
					"operationId": "createSession",
					"responses":   map[string]any{"201": map[string]any{"description": "Session created"}},
				},
			},
			"/v1/sessions/{id}": map[string]any{
				"get": map[string]any{
					"summary":     "Get session",
					"operationId": "getSession",
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]string{"type": "string"},
						},
					},
					"responses": map[string]any{"200": map[string]any{"description": "Session"}},
				},
				"delete": map[string]any{
					"summary":     "Delete session",
					"operationId": "deleteSession",
					"parameters": []map[string]any{
						{
							"name":     "id",
							"in":       "path",
							"required": true,
							"schema":   map[string]string{"type": "string"},
						},
					},
					"responses": map[string]any{"200": map[string]any{"description": "Deleted"}},
				},
			},
			"/v1/chat": map[string]any{
				"post": map[string]any{
					"summary":     "Send chat prompt",
					"operationId": "chat",
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"prompt":     map[string]string{"type": "string"},
										"session_id": map[string]string{"type": "string"},
									},
								},
							},
						},
					},
					"responses": map[string]any{"200": map[string]any{"description": "Chat response"}},
				},
			},
			"/v1/chat/stream": map[string]any{
				"post": map[string]any{
					"summary":     "Send chat prompt (SSE stream)",
					"operationId": "chatStream",
					"responses":   map[string]any{"200": map[string]any{"description": "SSE stream"}},
				},
			},
			"/v1/admin/keys": map[string]any{
				"get": map[string]any{
					"summary":     "List API keys (admin)",
					"operationId": "listKeys",
					"responses":   map[string]any{"200": map[string]any{"description": "Key list"}},
					"security":    []map[string]any{{"AdminKey": []string{}}},
				},
				"post": map[string]any{
					"summary":     "Create API key (admin)",
					"operationId": "createKey",
					"responses":   map[string]any{"201": map[string]any{"description": "Key created"}},
					"security":    []map[string]any{{"AdminKey": []string{}}},
				},
			},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"ApiKeyAuth": map[string]any{
					"type":   "http",
					"scheme": "bearer",
				},
				"AdminKey": map[string]any{
					"type":   "http",
					"scheme": "bearer",
				},
			},
		},
		"security": []map[string]any{
			{"ApiKeyAuth": []string{}},
		},
	}

	c.JSON(200, spec)
}
