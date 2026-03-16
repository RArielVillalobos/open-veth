package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// randID generates a random ID with the given prefix, e.g. "node-a1b2c3d4"
func randID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// HandleExport exports a laboratory topology to YAML
func (h *Handler) HandleExport(c *gin.Context) {
	labID := c.DefaultQuery("lab_id", "lab-1")

	nodes, err := h.Repo.ListNodesByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list nodes for export", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export topology"})
		return
	}
	links, err := h.Repo.ListLinksByLab(labID)
	if err != nil {
		h.Logger.Error("failed to list links for export", "lab", labID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export topology"})
		return
	}
	lab, ok := h.Repo.GetLaboratory(labID)
	if !ok {
		h.Logger.Warn("laboratory not found for export, using empty name", "lab", labID)
	}

	export := models.LabExport{
		Name:  lab.Name,
		Nodes: make([]models.NodeExport, 0, len(nodes)),
		Links: make([]models.LinkExport, 0, len(links)),
	}

	if export.Name == "" {
		export.Name = "Default Lab"
	}

	// Build lookup maps for interface and route configs
	ifaceConfigs, err := h.Repo.GetInterfaceConfigsByLab(labID)
	if err != nil {
		h.Logger.Warn("failed to get interface configs for export", "lab", labID, "error", err)
	}
	routeConfigs, err := h.Repo.GetRouteConfigsByLab(labID)
	if err != nil {
		h.Logger.Warn("failed to get route configs for export", "lab", labID, "error", err)
	}

	ifacesByNode := make(map[string][]models.InterfaceExport)
	for _, cfg := range ifaceConfigs {
		ifacesByNode[cfg.NodeID] = append(ifacesByNode[cfg.NodeID], models.InterfaceExport{
			Interface: cfg.Interface,
			Address:   cfg.Address,
		})
	}
	routesByNode := make(map[string][]models.RouteExport)
	for _, cfg := range routeConfigs {
		routesByNode[cfg.NodeID] = append(routesByNode[cfg.NodeID], models.RouteExport{
			Dst:     cfg.Dst,
			Gateway: cfg.Gateway,
			Dev:     cfg.Dev,
		})
	}

	// nodeID → name map for resolving link endpoints
	nodeNameByID := make(map[string]string, len(nodes))
	for _, n := range nodes {
		nodeNameByID[n.ID] = n.Name
		export.Nodes = append(export.Nodes, models.NodeExport{
			Name:       n.Name,
			Type:       n.Type,
			X:          n.X,
			Y:          n.Y,
			Interfaces: ifacesByNode[n.ID],
			Routes:     routesByNode[n.ID],
		})
	}

	for _, l := range links {
		export.Links = append(export.Links, models.LinkExport{
			Source:    nodeNameByID[l.SourceID],
			Target:    nodeNameByID[l.TargetID],
			SourceInt: l.SourceInt,
			TargetInt: l.TargetInt,
			Enabled:   l.Enabled,
		})
	}

	yamlData, err := yaml.Marshal(&export)
	if err != nil {
		h.Logger.Error("failed to generate YAML", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate YAML"})
		return
	}

	h.Logger.Info("exported topology", "lab", labID, "nodes", len(nodes), "links", len(links))

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, export.Name))
	c.Data(http.StatusOK, "application/x-yaml", yamlData)
}

// HandleImport imports a laboratory topology from YAML
func (h *Handler) HandleImport(c *gin.Context) {
	var imported models.LabExport

	// 1. Parse YAML from request body
	if err := c.ShouldBindYAML(&imported); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid YAML format: " + err.Error()})
		return
	}

	labID := c.DefaultQuery("lab_id", "lab-1")

	// Validate node types before touching any state
	for _, n := range imported.Nodes {
		if !models.IsValidNodeType(n.Type) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unknown node type: %q", n.Type)})
			return
		}
	}

	// Validate link references (all source/target names must exist in the node list)
	nodeNames := make(map[string]bool, len(imported.Nodes))
	for _, n := range imported.Nodes {
		nodeNames[n.Name] = true
	}
	for _, l := range imported.Links {
		if !nodeNames[l.Source] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("link references unknown node: %q", l.Source)})
			return
		}
		if !nodeNames[l.Target] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("link references unknown node: %q", l.Target)})
			return
		}
	}

	// Verify the lab exists before touching anything
	if _, ok := h.Repo.GetLaboratory(labID); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("laboratory %q not found", labID)})
		return
	}

	h.Logger.Info("importing topology", "name", imported.Name, "nodes", len(imported.Nodes), "links", len(imported.Links))

	// 2. Cleanup current state (Containers and DB)
	ctx := c.Request.Context()
	existingNodes, _ := h.Repo.ListNodesByLab(labID)
	for _, n := range existingNodes {
		if err := h.Manager.DeleteNode(ctx, n.Name); err != nil {
			h.Logger.Warn("failed to delete node during import cleanup", "node", n.Name, "error", err)
		}
		h.Runtime.Delete(n.ID)
	}

	// Capture existing lab before deleting so we can restore on failure
	existingLab, _ := h.Repo.GetLaboratory(labID)
	if err := h.Repo.DeleteLaboratory(labID); err != nil {
		h.Logger.Warn("failed to delete old lab during import", "lab", labID, "error", err)
	}
	if err := h.Repo.SaveLaboratory(models.Laboratory{ID: labID, Name: imported.Name}); err != nil {
		h.Logger.Error("failed to create lab during import", "lab", labID, "error", err)
		// Best-effort restore so the lab isn't left in a deleted state
		_ = h.Repo.SaveLaboratory(existingLab)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create laboratory"})
		return
	}

	// 3. Recreate Nodes — generate fresh IDs, build name→ID map
	var errors []string
	nameToID := make(map[string]string, len(imported.Nodes))

	for _, n := range imported.Nodes {
		nodeModel := models.Node{
			ID:    randID("node-"),
			LabID: labID,
			Name:  n.Name,
			Type:  n.Type,
			Image: models.GetImageForType(n.Type), // SECURITY: Force official image
			X:     n.X,
			Y:     n.Y,
		}

		containerID, err := h.Manager.CreateNode(ctx, nodeModel)
		if err != nil {
			h.Logger.Warn("import node failed", "node", n.Name, "error", err)
			errors = append(errors, fmt.Sprintf("Failed to create node %s: %v", n.Name, err))
			continue
		}

		pid, err := h.Manager.GetNodePID(ctx, containerID)
		if err != nil {
			h.Logger.Warn("failed to get PID during import", "node", n.Name, "error", err)
		}
		h.Runtime.Set(nodeModel.ID, containerID, pid)
		nodeModel.ContainerID = containerID
		nodeModel.PID = pid
		if err := h.Repo.SaveNode(nodeModel); err != nil {
			h.Logger.Error("import node save failed", "node", n.Name, "error", err)
			errors = append(errors, fmt.Sprintf("Failed to save node %s: %v", n.Name, err))
			continue
		}
		nameToID[n.Name] = nodeModel.ID
	}

	// 4. Recreate Links — resolve source/target names to IDs
	for _, l := range imported.Links {
		sourceID, okS := nameToID[l.Source]
		targetID, okT := nameToID[l.Target]
		if !okS || !okT {
			errors = append(errors, fmt.Sprintf("Link %s→%s skipped: one or both nodes failed to create", l.Source, l.Target))
			continue
		}

		linkModel := models.Link{
			ID:        randID("link-"),
			LabID:     labID,
			SourceID:  sourceID,
			TargetID:  targetID,
			SourceInt: l.SourceInt,
			TargetInt: l.TargetInt,
			Enabled:   l.Enabled,
		}

		source, okS := h.Repo.GetNode(sourceID)
		target, okT := h.Repo.GetNode(targetID)
		if okS {
			h.hydrateNode(&source)
		}
		if okT {
			h.hydrateNode(&target)
		}

		if okS && okT {
			if err := h.Network.CreateLink(linkModel, source.PID, target.PID); err != nil {
				h.Logger.Warn("import link failed", "source", l.Source, "target", l.Target, "error", err)
				errors = append(errors, fmt.Sprintf("Failed to create link %s→%s: %v", l.Source, l.Target, err))
			} else {
				if models.NeedsBridge(source.Type) {
					_ = h.Manager.AttachInterfaceToBridge(ctx, source.ContainerID, linkModel.SourceInt, source.Type)
				}
				if models.NeedsBridge(target.Type) {
					_ = h.Manager.AttachInterfaceToBridge(ctx, target.ContainerID, linkModel.TargetInt, target.Type)
				}
				if err := h.Repo.SaveLink(linkModel); err != nil {
					h.Logger.Error("import link save failed", "source", l.Source, "target", l.Target, "error", err)
					errors = append(errors, fmt.Sprintf("Failed to save link %s→%s: %v", l.Source, l.Target, err))
				}
			}
		} else {
			errors = append(errors, fmt.Sprintf("Link %s→%s skipped: source or target not found in repo", l.Source, l.Target))
		}
	}

	// 5. Apply and persist interface/route configs
	var ifaceConfigs []models.InterfaceConfig
	var routeConfigs []models.RouteConfig

	for _, n := range imported.Nodes {
		nodeID, ok := nameToID[n.Name]
		if !ok {
			continue // node failed to create
		}
		node, ok := h.Repo.GetNode(nodeID)
		if !ok {
			continue
		}
		h.hydrateNode(&node)

		for _, iface := range n.Interfaces {
			ifaceConfigs = append(ifaceConfigs, models.InterfaceConfig{
				LabID:     labID,
				NodeID:    nodeID,
				Interface: iface.Interface,
				Address:   iface.Address,
			})
			if node.ContainerID != "" {
				if err := h.Manager.ConfigureInterface(ctx, node.ContainerID, iface.Interface, iface.Address); err != nil {
					h.Logger.Warn("failed to apply interface config during import", "node", n.Name, "iface", iface.Interface, "error", err)
					errors = append(errors, fmt.Sprintf("Failed to configure %s on %s: %v", iface.Interface, n.Name, err))
				}
			}
		}

		for _, route := range n.Routes {
			routeConfigs = append(routeConfigs, models.RouteConfig{
				LabID:   labID,
				NodeID:  nodeID,
				Dst:     route.Dst,
				Gateway: route.Gateway,
				Dev:     route.Dev,
			})
			if node.ContainerID != "" {
				if err := h.Manager.ConfigureRoute(ctx, node.ContainerID, route.Dst, route.Gateway, route.Dev); err != nil {
					h.Logger.Warn("failed to apply route config during import", "node", n.Name, "dst", route.Dst, "error", err)
					errors = append(errors, fmt.Sprintf("Failed to add route %s on %s: %v", route.Dst, n.Name, err))
				}
			}
		}
	}

	if len(ifaceConfigs) > 0 {
		if err := h.Repo.SaveInterfaceConfigs(labID, ifaceConfigs); err != nil {
			h.Logger.Warn("failed to persist interface configs during import", "error", err)
		}
	}
	if len(routeConfigs) > 0 {
		if err := h.Repo.SaveRouteConfigs(labID, routeConfigs); err != nil {
			h.Logger.Warn("failed to persist route configs during import", "error", err)
		}
	}

	status := http.StatusOK
	if len(errors) > 0 {
		status = http.StatusMultiStatus
	}

	h.Logger.Info("import completed", "nodes", len(imported.Nodes), "links", len(imported.Links), "errors", len(errors))

	c.JSON(status, gin.H{
		"message": "import process completed",
		"nodes":   len(imported.Nodes),
		"links":   len(imported.Links),
		"errors":  errors,
	})
}
