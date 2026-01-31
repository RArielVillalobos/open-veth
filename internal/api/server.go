package api

import (
	"net/http"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server encapsula el router HTTP y las dependencias
type Server struct {
	router  *gin.Engine
	manager *orchestrator.Manager
	repo    storage.Repository
}

// NewServer crea y configura una instancia del servidor API
func NewServer(mgr *orchestrator.Manager) *Server {
	r := gin.Default()

	// Configuración CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AddAllowHeaders("Authorization")
	r.Use(cors.New(config))

	s := &Server{
		router:  r,
		manager: mgr,
		repo:    storage.NewMemoryRepository(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes define los endpoints granulares (Fase 4)
func (s *Server) setupRoutes() {
	// Health Check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "0.4-realtime"})
	})

	api := s.router.Group("/api/v1")
	{
		// Terminal (Websocket)
		api.GET("/terminal", s.handleTerminal)

		// Nodos
		api.GET("/nodes", s.listNodes)
		api.POST("/nodes", s.createNode)
		api.DELETE("/nodes/:id", s.deleteNode)
		
		// Links
		api.GET("/links", s.listLinks)
		api.POST("/links", s.createLink)
		api.DELETE("/links/:id", s.deleteLink)

		// Limpieza Global
		api.DELETE("/system/cleanup", s.handleCleanup)
	}
}

// Run arranca el servidor
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// --- Handlers de Nodos ---

func (s *Server) listNodes(c *gin.Context) {
	nodes, _ := s.repo.ListNodes()
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