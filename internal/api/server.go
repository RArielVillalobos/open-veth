package api

import (
	"net/http"
	"open-veth/internal/orchestrator"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server encapsula el router HTTP y las dependencias
type Server struct {
	router  *gin.Engine
	manager *orchestrator.Manager
}

// NewServer crea y configura una instancia del servidor API
func NewServer(mgr *orchestrator.Manager) *Server {
	r := gin.Default()

	// Configuración CORS (Permisiva para desarrollo)
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AddAllowHeaders("Authorization")
	r.Use(cors.New(config))

	s := &Server{
		router:  r,
		manager: mgr,
	}

	s.setupRoutes()
	return s
}

// setupRoutes define los endpoints
func (s *Server) setupRoutes() {
	// Health Check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "0.2"})
	})

	api := s.router.Group("/api/v1")
	{
		// Terminal (Websocket)
		api.GET("/terminal", s.handleTerminal)

		// Topología
		api.POST("/topology/deploy", s.handleDeployTopology)
		api.DELETE("/topology/cleanup", s.handleCleanup)
	}
}

// Run arranca el servidor
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// --- Handlers Temporales ---

func (s *Server) handleDeployTopology(c *gin.Context) {
	// Aquí parsearemos el JSON del frontend
	c.JSON(http.StatusOK, gin.H{"message": "Deploy endpoint (Not implemented yet)"})
}

func (s *Server) handleCleanup(c *gin.Context) {
	// Aquí llamaremos a manager.DeleteNode para todo
	c.JSON(http.StatusOK, gin.H{"message": "Cleanup endpoint (Not implemented yet)"})
}
