package handlers

import (
	"fmt"
	"net/http"

	"open-veth/internal/models"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// HandleExport exports a laboratory topology to YAML
func (h *Handler) HandleExport(c *gin.Context) {
	labID := c.DefaultQuery("lab_id", "lab-1")

	nodes, _ := h.Repo.ListNodesByLab(labID)
	links, _ := h.Repo.ListLinksByLab(labID)
	lab, _ := h.Repo.GetLaboratory(labID)

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
		h.Logger.Error("failed to generate YAML", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate YAML"})
		return
	}

	h.Logger.Info("exported topology", "lab", labID, "nodes", len(nodes), "links", len(links))

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.yaml", export.Name))
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

	labID := "lab-1" // Current limitation: single lab

	h.Logger.Info("importing topology", "name", imported.Name, "nodes", len(imported.Nodes), "links", len(imported.Links))

	// 2. Cleanup current state (Containers and DB)
	ctx := c.Request.Context()
	nodes, _ := h.Repo.ListNodesByLab(labID)
	for _, n := range nodes {
		if err := h.Manager.DeleteNode(ctx, n.Name); err != nil {
			h.Logger.Warn("failed to delete node during import cleanup", "node", n.Name, "error", err)
		}
	}

	h.Repo.DeleteLaboratory(labID)
	h.Repo.SaveLaboratory(models.Laboratory{ID: labID, Name: imported.Name})

	// 3. Recreate Nodes
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
		containerID, err := h.Manager.CreateNode(ctx, nodeModel)
		if err != nil {
			msg := fmt.Sprintf("Failed to create node %s: %v", n.Name, err)
			h.Logger.Warn("import node failed", "node", n.Name, "error", err)
			errors = append(errors, msg)
			continue
		}

		pid, _ := h.Manager.GetNodePID(ctx, containerID)
		nodeModel.ContainerID = containerID
		nodeModel.PID = pid
		h.Repo.SaveNode(nodeModel)
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

		source, okS := h.Repo.GetNode(l.Source)
		target, okT := h.Repo.GetNode(l.Target)

		if okS && okT {
			if err := h.Network.CreateLink(linkModel, source.PID, target.PID); err != nil {
				msg := fmt.Sprintf("Failed to create link %s: %v", l.ID, err)
				h.Logger.Warn("import link failed", "link", l.ID, "error", err)
				errors = append(errors, msg)
			} else {
				// Attach to Bridge if Switch
				if source.Type == models.SWITCH {
					_ = h.Manager.AttachInterfaceToBridge(ctx, source.ContainerID, linkModel.SourceInt)
				}
				if target.Type == models.SWITCH {
					_ = h.Manager.AttachInterfaceToBridge(ctx, target.ContainerID, linkModel.TargetInt)
				}
				h.Repo.SaveLink(linkModel)
			}
		} else {
			errors = append(errors, fmt.Sprintf("Link %s skipped: source or target not found", l.ID))
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
