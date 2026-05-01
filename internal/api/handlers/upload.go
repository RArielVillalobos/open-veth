package handlers

import (
	"archive/tar"
	"bytes"
	"net/http"
	"path"

	"github.com/docker/docker/api/types/container"
	"github.com/gin-gonic/gin"
)

// UploadFile receives a file via multipart form and copies it into the node's container.
// Query param: path (optional, default /root)
func (h *Handler) UploadFile(c *gin.Context) {
	nodeID := c.Param("id")

	node, ok := h.getRunningNode(c, nodeID)
	if !ok {
		return
	}

	// Get destination path (default: /root) — must be an absolute path
	destPath := c.DefaultQuery("path", "/root")
	if len(destPath) == 0 || destPath[0] != '/' {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path must be an absolute path"})
		return
	}

	// Limit upload size to 50MB
	const maxUploadSize = 50 << 20 // 50MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	// Parse uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	// Read file content
	var fileBuf bytes.Buffer
	if _, err := fileBuf.ReadFrom(file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	// Wrap in a tar archive (Docker CopyToContainer requires tar format)
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{
		Name: path.Base(header.Filename),
		Mode: 0644,
		Size: int64(fileBuf.Len()),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tar"})
		return
	}
	if _, err := tw.Write(fileBuf.Bytes()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write tar"})
		return
	}
	tw.Close()

	// Copy tar into container
	err = h.Manager.GetDockerClient().CopyToContainer(
		c.Request.Context(),
		node.ContainerID,
		destPath,
		&tarBuf,
		container.CopyToContainerOptions{},
	)
	if err != nil {
		h.Logger.Error("failed to copy file to container", "node", node.Name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy file: " + err.Error()})
		return
	}

	h.Logger.Info("file uploaded to node", "node", node.Name, "file", header.Filename, "path", destPath)
	c.JSON(http.StatusOK, gin.H{
		"message":  "file uploaded successfully",
		"filename": header.Filename,
		"path":     destPath + "/" + header.Filename,
	})
}
