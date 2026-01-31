package api

import (
	"fmt"
	"net/http"
	"os"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server encapsulates the HTTP router and dependencies
type Server struct {
	router  *gin.Engine
	manager *orchestrator.Manager
	repo    storage.Repository
}

// NewServer creates and configures the API server instance
func NewServer(mgr *orchestrator.Manager) *Server {
	r := gin.Default()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AddAllowHeaders("Authorization")
	r.Use(cors.New(config))

	// Database Configuration from Environment
	dbDriver := os.Getenv("DB_DRIVER")
	if dbDriver == "" {
		dbDriver = "sqlite"
	}
	dbDSN := os.Getenv("DB_DSN")
	if dbDSN == "" {
		dbDSN = "openveth.db"
	}

	// Initialize Repository
	var repo storage.Repository
	dbRepo, err := storage.NewGormRepository(dbDriver, dbDSN)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize DB (%s), falling back to Memory: %v\n", dbDriver, err)
		repo = storage.NewMemoryRepository()
	} else {
		repo = dbRepo
	}

	s := &Server{
		router:  r,
		manager: mgr,
		repo:    repo,
	}

	s.setupRoutes()
	return s
}

// setupRoutes defines granular endpoints (Phase 4)
func (s *Server) setupRoutes() {
	// Health Check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "0.4-realtime"})
	})

	api := s.router.Group("/api/v1")
	{
		// Terminal (Websocket)
		api.GET("/terminal", s.handleTerminal)

		// Nodes
		api.GET("/nodes", s.listNodes)
		api.POST("/nodes", s.createNode)
		api.DELETE("/nodes/:id", s.deleteNode)
		api.GET("/nodes/:id/interfaces", s.getNodeInterfaces) // New Real-Time endpoint
		
		// Links
		api.GET("/links", s.listLinks)
		api.POST("/links", s.createLink)
		api.DELETE("/links/:id", s.deleteLink)

		// Global Cleanup
		api.DELETE("/system/cleanup", s.handleCleanup)
	}
}

// Run starts the server
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// --- Node Handlers ---

func (s *Server) listNodes(c *gin.Context) {
	nodes, _ := s.repo.ListNodes()

	// If real-time info is requested
	if c.Query("live") == "true" {
		for i := range nodes {
			if nodes[i].ContainerID != "" {
				if ifaces, err := s.manager.GetNodeInterfaces(c.Request.Context(), nodes[i].ContainerID); err == nil {
					nodes[i].Interfaces = ifaces
				}
			}
		}
	}

	c.JSON(http.StatusOK, nodes)
}

func (s *Server) createNode(c *gin.Context) {
	var node models.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	containerID, err := s.manager.CreateNode(c.Request.Context(), node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pid, err := s.manager.GetNodePID(c.Request.Context(), containerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	node.ContainerID = containerID
	node.PID = pid
	s.repo.SaveNode(node)

	c.JSON(http.StatusCreated, node)
}

func (s *Server) deleteNode(c *gin.Context) {
	id := c.Param("id")
	node, found := s.repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	_ = s.manager.DeleteNode(c.Request.Context(), node.Name)
	s.repo.DeleteNode(id)
	c.Status(http.StatusNoContent)
}

func (s *Server) getNodeInterfaces(c *gin.Context) {
	id := c.Param("id")
	node, found := s.repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	if node.ContainerID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node is not running"})
		return
	}

	interfaces, err := s.manager.GetNodeInterfaces(c.Request.Context(), node.ContainerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, interfaces)
}

// --- Handlers de Links ---

func (s *Server) listLinks(c *gin.Context) {
	links, _ := s.repo.ListLinks()
	c.JSON(http.StatusOK, links)
}

func (s *Server) createLink(c *gin.Context) {
	var link models.Link
	if err := c.ShouldBindJSON(&link); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	source, okS := s.repo.GetNode(link.SourceID)
	target, okT := s.repo.GetNode(link.TargetID)

	if !okS || !okT {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source or target node not found"})
		return
	}

	// Validation: Check for existing link between these nodes
	existingLinks, _ := s.repo.ListLinks()
	for _, l := range existingLinks {
		// Check both directions
		if (l.SourceID == link.SourceID && l.TargetID == link.TargetID) ||
		   (l.SourceID == link.TargetID && l.TargetID == link.SourceID) {
			c.JSON(http.StatusConflict, gin.H{"error": "link already exists between these nodes"})
			return
		}
	}

	nm := orchestrator.NewNetworkManager()
	if err := nm.CreateLink(link, source.PID, target.PID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.repo.SaveLink(link)
	c.JSON(http.StatusCreated, link)
}

func (s *Server) deleteLink(c *gin.Context) {
	id := c.Param("id")
	s.repo.DeleteLink(id)
	c.Status(http.StatusNoContent)
}

// handleCleanup elimina todos los contenedores con label openveth=true
func (s *Server) handleCleanup(c *gin.Context) {
	ctx := c.Request.Context()
	containers, _ := s.manager.GetDockerClient().ContainerList(ctx, container.ListOptions{All: true})

	for _, ct := range containers {
		if ct.Labels["openveth"] == "true" {
			_ = s.manager.GetDockerClient().ContainerRemove(ctx, ct.ID, container.RemoveOptions{Force: true})
		}
	}

	s.repo.ClearAll()
	c.JSON(http.StatusOK, gin.H{"message": "cleanup complete"})
}