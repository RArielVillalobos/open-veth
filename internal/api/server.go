package api

import (
	"log"
	"net/http"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"

	"github.com/docker/docker/api/types/container"
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

// handleDeployTopology procesa el despliegue de una topología completa
func (s *Server) handleDeployTopology(c *gin.Context) {
	var topo models.Topology
	if err := c.ShouldBindJSON(&topo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Formato JSON inválido", "details": err.Error()})
		return
	}

	ctx := c.Request.Context()
	nodePIDs := make(map[string]int) 

	// 1. Desplegar Nodos
	for i, node := range topo.Nodes {
		containerID, err := s.manager.CreateNode(ctx, node)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creando nodo " + node.Name, "details": err.Error()})
			return
		}

		pid, err := s.manager.GetNodePID(ctx, containerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error obteniendo PID de " + node.Name, "details": err.Error()})
			return
		}
		
		nodePIDs[node.ID] = pid
		topo.Nodes[i].ContainerID = containerID
		topo.Nodes[i].PID = pid
	}

	// 2. Desplegar Enlaces
	netManager := orchestrator.NewNetworkManager()
	for _, link := range topo.Links {
		pidSource, okS := nodePIDs[link.SourceID]
		pidTarget, okT := nodePIDs[link.TargetID]

		if !okS || !okT {
			log.Printf("Link %s omitido: nodos no encontrados", link.ID)
			continue
		}

		if err := netManager.CreateLink(link, pidSource, pidTarget); err != nil {
			log.Printf("Error creando link %s: %v", link.ID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Topología desplegada exitosamente",
		"nodes_count": len(topo.Nodes),
		"links_count": len(topo.Links),
	})
}

// handleCleanup elimina todos los contenedores gestionados
func (s *Server) handleCleanup(c *gin.Context) {
	ctx := c.Request.Context()
	
	containers, err := s.manager.GetDockerClient().ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error listando", "details": err.Error()})
		return
	}

	count := 0
	for _, ct := range containers {
		if ct.Labels["openveth"] == "true" {
			err := s.manager.GetDockerClient().ContainerRemove(ctx, ct.ID, container.RemoveOptions{Force: true})
			if err == nil { count++ }
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Limpieza completada", "removed_count": count})
}