package api

import (
	"context"
	"fmt"
	"net/http"
	"open-veth/internal/models"
	"open-veth/internal/orchestrator"
	"open-veth/internal/storage"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
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
		api.PATCH("/nodes/:id/position", s.updateNodePosition) // New for auto-save
		api.DELETE("/nodes/:id", s.deleteNode)
		api.GET("/nodes/:id/interfaces", s.getNodeInterfaces) // New Real-Time endpoint

		// Links
		api.GET("/links", s.listLinks)
		api.POST("/links", s.createLink)
		api.DELETE("/links/:id", s.deleteLink)

		// Laboratories
		api.GET("/laboratories", s.listLaboratories)
		api.POST("/laboratories", s.createLaboratory)
		api.POST("/laboratories/:id/activate", s.activateLaboratory) // New: Swap Labs
		api.PATCH("/laboratories/:id", s.updateLaboratory)
		api.DELETE("/laboratories/:id", s.deleteLaboratory)

		// Topology Export/Import
		api.GET("/topology/export", s.handleExport)
		api.POST("/topology/import", s.handleImport)

		// Global Cleanup
		api.DELETE("/system/cleanup", s.handleCleanup)
	}
}

// Run starts the HTTP server
func (s *Server) Run(addr string) error {
	// Perform startup reconciliation (The Janitor)
	fmt.Println("🧹 Running startup reconciliation...")
	if err := s.reconcileState(); err != nil {
		fmt.Printf("Warning: reconciliation failed: %v\n", err)
	}

	return s.router.Run(addr)
}

// reconcileState ensures Docker matches the Database state
func (s *Server) reconcileState() error {
	ctx := context.Background()

	// 1. Get all OpenVeth containers (The Reality)
	containers, err := s.manager.GetOpenVethContainers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list docker containers: %v", err)
	}

	// 2. Get all Nodes from DB (The Desired State)
	// We need to list ALL nodes from ALL labs to know what is valid
	nodes, err := s.repo.ListNodes()
	if err != nil {
		return fmt.Errorf("failed to list db nodes: %v", err)
	}

	// 3. Create a lookup map for valid container IDs
	validContainers := make(map[string]bool)
	for _, node := range nodes {
		// We use Name as the key identifier since ContainerID might change if recreated manually,
		// but openveth relies heavily on naming conventions.
		// Ideally we should check both, but let's stick to Name for now as it maps to container name.
		validContainers["/"+node.Name] = true // Docker names often start with /
		validContainers[node.Name] = true
	}

	// 4. Hunt for Zombies
	zombieCount := 0
	for _, container := range containers {
		isZombie := true
		for _, name := range container.Names {
			if validContainers[name] {
				isZombie = false
				break
			}
		}

		if isZombie {
			fmt.Printf("🧟 Zombie detected: %v (ID: %s). Terminating...\n", container.Names, container.ID[:12])
			// Use the orchestrator to delete it properly
			// We can pass the container ID or Name. Manager.DeleteNode expects Name usually but ContainerRemove works with ID.
			// Let's use ID for precision here.
			if err := s.manager.DeleteNode(ctx, container.ID); err != nil {
				fmt.Printf("Failed to kill zombie %s: %v\n", container.ID[:12], err)
			} else {
				zombieCount++
			}
		}
	}

	if zombieCount > 0 {
		fmt.Printf("✨ Cleaned up %d zombie containers.\n", zombieCount)
	} else {
		fmt.Println("✅ System state is clean.")
	}

	return nil
}

// --- Topology Export/Import Handlers ---

func (s *Server) handleImport(c *gin.Context) {
	var imported models.LabExport

	// 1. Parse YAML from request body
	if err := c.ShouldBindYAML(&imported); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid YAML format: " + err.Error()})
		return
	}

	labID := "lab-1" // Current limitation: single lab

	// 2. Cleanup current state (Containers and DB)
	// We reuse the logic from handleCleanup but targeted to this lab
	ctx := c.Request.Context()
	nodes, _ := s.repo.ListNodesByLab(labID)
	for _, n := range nodes {
		_ = s.manager.DeleteNode(ctx, n.Name)
	}

	// We don't use s.repo.ClearAll() because we might have other labs in the future.
	// For now, we manually delete nodes/links of this lab.
	// Optimization: Since it's lab-1, we can just clear if we assume single lab.
	s.repo.DeleteLaboratory(labID)
	s.repo.SaveLaboratory(models.Laboratory{ID: labID, Name: imported.Name})

	// 3. Recreate Nodes
	nm := orchestrator.NewNetworkManager()
	var errors []string

	for _, n := range imported.Nodes {
		nodeModel := models.Node{
			ID:    n.ID,
			LabID: labID,
			Name:  n.Name,
			Type:  n.Type,
			Image: models.GetImageForType(n.Type), // SECURITY: Force official image
			X:     n.X,
			Y:     n.Y,
		}

		// Create container
		containerID, err := s.manager.CreateNode(ctx, nodeModel)
		if err != nil {
			msg := fmt.Sprintf("Failed to create node %s: %v", n.Name, err)
			fmt.Printf("Warning: %s\n", msg)
			errors = append(errors, msg)
			continue
		}

		pid, _ := s.manager.GetNodePID(ctx, containerID)
		nodeModel.ContainerID = containerID
		nodeModel.PID = pid
		s.repo.SaveNode(nodeModel)
	}

	// 4. Recreate Links
	for _, l := range imported.Links {
		linkModel := models.Link{
			ID:        l.ID,
			LabID:     labID,
			SourceID:  l.Source,
			TargetID:  l.Target,
			SourceInt: l.SourceInt,
			TargetInt: l.TargetInt,
		}

		source, okS := s.repo.GetNode(l.Source)
		target, okT := s.repo.GetNode(l.Target)

		if okS && okT {
			if err := nm.CreateLink(linkModel, source.PID, target.PID); err != nil {
				msg := fmt.Sprintf("Failed to create link %s: %v", l.ID, err)
				fmt.Printf("Warning: %s\n", msg)
				errors = append(errors, msg)
			} else {
				// Attach to Bridge if Switch
				if source.Type == models.SWITCH {
					_ = s.manager.AttachInterfaceToBridge(ctx, source.ContainerID, linkModel.SourceInt)
				}
				if target.Type == models.SWITCH {
					_ = s.manager.AttachInterfaceToBridge(ctx, target.ContainerID, linkModel.TargetInt)
				}
				s.repo.SaveLink(linkModel)
			}
		} else {
			errors = append(errors, fmt.Sprintf("Link %s skipped: source or target not found", l.ID))
		}
	}

	status := http.StatusOK
	if len(errors) > 0 {
		status = http.StatusMultiStatus
	}

	c.JSON(status, gin.H{
		"message": "import process completed",
		"nodes":   len(imported.Nodes),
		"links":   len(imported.Links),
		"errors":  errors,
	})
}

func (s *Server) handleExport(c *gin.Context) {
	labID := c.DefaultQuery("lab_id", "lab-1")

	nodes, _ := s.repo.ListNodesByLab(labID)
	links, _ := s.repo.ListLinksByLab(labID)
	lab, _ := s.repo.GetLaboratory(labID)

	export := models.LabExport{
		Version: "0.4",
		Name:    lab.Name,
		Nodes:   make([]models.NodeExport, 0, len(nodes)),
		Links:   make([]models.LinkExport, 0, len(links)),
	}

	if export.Name == "" {
		export.Name = "Default Lab"
	}

	for _, n := range nodes {
		export.Nodes = append(export.Nodes, models.NodeExport{
			ID:   n.ID,
			Name: n.Name,
			Type: n.Type,
			X:    n.X,
			Y:    n.Y,
		})
	}

	for _, l := range links {
		export.Links = append(export.Links, models.LinkExport{
			ID:        l.ID,
			Source:    l.SourceID,
			Target:    l.TargetID,
			SourceInt: l.SourceInt,
			TargetInt: l.TargetInt,
		})
	}

	yamlData, err := yaml.Marshal(&export)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate YAML"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.yaml", export.Name))
	c.Data(http.StatusOK, "application/x-yaml", yamlData)
}

// --- Node Handlers ---

func (s *Server) listNodes(c *gin.Context) {
	labID := c.Query("lab_id")
	var nodes []models.Node
	var err error

	if labID != "" {
		nodes, err = s.repo.ListNodesByLab(labID)
	} else {
		nodes, err = s.repo.ListNodes()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

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

	// Default LabID if not provided
	if node.LabID == "" {
		node.LabID = "lab-1"
	}

	// SECURITY: Override user-provided image with official image
	node.Image = models.GetImageForType(node.Type)

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

func (s *Server) updateNodePosition(c *gin.Context) {
	id := c.Param("id")
	var pos struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}

	if err := c.ShouldBindJSON(&pos); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node, found := s.repo.GetNode(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	node.X = pos.X
	node.Y = pos.Y
	s.repo.SaveNode(node)

	c.Status(http.StatusNoContent)
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

// --- Link Handlers ---

func (s *Server) listLinks(c *gin.Context) {
	labID := c.Query("lab_id")
	var links []models.Link
	var err error

	if labID != "" {
		links, err = s.repo.ListLinksByLab(labID)
	} else {
		links, err = s.repo.ListLinks()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, links)
}

func (s *Server) createLink(c *gin.Context) {
	var link models.Link
	if err := c.ShouldBindJSON(&link); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default LabID if not provided
	if link.LabID == "" {
		link.LabID = "lab-1"
	}

	source, okS := s.repo.GetNode(link.SourceID)
	target, okT := s.repo.GetNode(link.TargetID)

	if !okS || !okT {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source or target node not found"})
		return
	}

	// Validation: Check for existing link between these nodes
	existingLinks, _ := s.repo.ListLinksByLab(link.LabID)
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

	// Attach interfaces to bridge if nodes are Switches
	if source.Type == models.SWITCH {
		if err := s.manager.AttachInterfaceToBridge(c.Request.Context(), source.ContainerID, link.SourceInt); err != nil {
			fmt.Printf("Error attaching interface %s to switch %s: %v\n", link.SourceInt, source.Name, err)
		}
	}
	if target.Type == models.SWITCH {
		if err := s.manager.AttachInterfaceToBridge(c.Request.Context(), target.ContainerID, link.TargetInt); err != nil {
			fmt.Printf("Error attaching interface %s to switch %s: %v\n", link.TargetInt, target.Name, err)
		}
	}

	s.repo.SaveLink(link)
	c.JSON(http.StatusCreated, link)
}

func (s *Server) deleteLink(c *gin.Context) {
	id := c.Param("id")
	s.repo.DeleteLink(id)
	c.Status(http.StatusNoContent)
}

// --- Laboratory Handlers ---

func (s *Server) listLaboratories(c *gin.Context) {
	labs, err := s.repo.ListLaboratories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, labs)
}

func (s *Server) createLaboratory(c *gin.Context) {
	var lab models.Laboratory
	if err := c.ShouldBindJSON(&lab); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if lab.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lab id is required"})
		return
	}

	if err := s.repo.SaveLaboratory(lab); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, lab)
}

func (s *Server) updateLaboratory(c *gin.Context) {
	id := c.Param("id")
	var data struct {
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lab, found := s.repo.GetLaboratory(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	lab.Name = data.Name
	s.repo.SaveLaboratory(lab)

	c.JSON(http.StatusOK, lab)
}

func (s *Server) deleteLaboratory(c *gin.Context) {
	id := c.Param("id")
	if err := s.repo.DeleteLaboratory(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) activateLaboratory(c *gin.Context) {
	labID := c.Param("id")
	ctx := c.Request.Context()

	// 1. Verify Lab Exists
	if _, ok := s.repo.GetLaboratory(labID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "laboratory not found"})
		return
	}

	// 2. NUKE: Delete ALL running containers to free resources
	// We reuse the logic of finding openveth containers
	containers, err := s.manager.GetOpenVethContainers(ctx)
	if err == nil {
		for _, container := range containers {
			// Don't error out, just try to kill everything
			_ = s.manager.DeleteNode(ctx, container.ID)
		}
	}

	// 3. Rebuild Nodes
	nodes, err := s.repo.ListNodesByLab(labID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch nodes: " + err.Error()})
		return
	}

	for i, n := range nodes {
		// Re-create container
		containerID, err := s.manager.CreateNode(ctx, n)
		if err != nil {
			fmt.Printf("Error reviving node %s: %v\n", n.Name, err)
			continue
		}

		// Update Runtime Info in Struct (PID, ID)
		pid, _ := s.manager.GetNodePID(ctx, containerID)
		nodes[i].ContainerID = containerID
		nodes[i].PID = pid

		// CRITICAL: Persist the new ContainerID and PID to DB
		// Otherwise, subsequent API calls (like getInterfaces) will look for the old container
		s.repo.SaveNode(nodes[i])
	}

	// 4. Rebuild Links
	links, err := s.repo.ListLinksByLab(labID)
	if err != nil {
		fmt.Printf("Error fetching links: %v\n", err)
	}

	nm := orchestrator.NewNetworkManager()

	for _, l := range links {
		// Find source and target PIDs from our fresh `nodes` slice
		var srcNode, tgtNode models.Node
		foundS, foundT := false, false

		for _, n := range nodes {
			if n.ID == l.SourceID {
				srcNode = n
				foundS = true
			}
			if n.ID == l.TargetID {
				tgtNode = n
				foundT = true
			}
		}

		if foundS && foundT && srcNode.PID > 0 && tgtNode.PID > 0 {
			if err := nm.CreateLink(l, srcNode.PID, tgtNode.PID); err != nil {
				fmt.Printf("Error reviving link %s: %v\n", l.ID, err)
			} else {
				// Re-attach bridges if needed
				if srcNode.Type == models.SWITCH {
					_ = s.manager.AttachInterfaceToBridge(ctx, srcNode.ContainerID, l.SourceInt)
				}
				if tgtNode.Type == models.SWITCH {
					_ = s.manager.AttachInterfaceToBridge(ctx, tgtNode.ContainerID, l.TargetInt)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "laboratory activated", "nodes_revived": len(nodes)})
}

// handleCleanup eliminates all containers with label openveth=true
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
