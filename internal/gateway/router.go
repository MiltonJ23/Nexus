package gateway

import (
	pb "Nexus/proto"
	auth_pb "Nexus/proto/auth"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	nexus pb.NexusControllerClient
	auth  auth_pb.AuthServiceClient
}

// NewServer initialise le serveur avec les clients gRPC injectés
// NewServer constructs a Server and injects the provided Nexus and Auth gRPC clients.
func NewServer(nexusClient pb.NexusControllerClient, authClient auth_pb.AuthServiceClient) *Server {
	return &Server{
		nexus: nexusClient,
		auth:  authClient,
	}
}

func (s *Server) SetupRouter() *gin.Engine {
	r := gin.Default()

	// Configuration CORS (Permet au frontend React/Vue/etc d'appeler l'API)
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// --- AUTH ROUTES (Publiques) ---
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register/init", s.handleRegisterInit)
		authGroup.POST("/register/verify", s.handleRegisterVerify)
		authGroup.POST("/login/init", s.handleLoginInit)
		authGroup.POST("/login/verify", s.handleLoginVerify)
	}

	// --- API ROUTES (Protégées par Middleware) ---
	api := r.Group("/api")
	api.Use(s.AuthMiddleware())
	{
		// Gestion des Nœuds
		api.POST("/nodes", s.handleCreateNode)
		// Note: Assure-toi que "ListNodes" existe dans ton nexus.proto, sinon commente cette ligne
		// api.GET("/nodes", s.handleListNodes)

		// Gestion des Fichiers
		api.POST("/files/upload", s.handleUpload)
		api.GET("/files", s.handleListFiles)
		api.GET("/files/:id/download", s.handleDownload)
		api.GET("/api/nodes/:id/metrics", s.handleGetMetrics)
		api.POST("/lambda/run", s.handleRunLambda)

		fsGroup := api.Group("/fs")
		{
			fsGroup.POST("/mkdir", s.handleFSMkdir)     // Create folder
			fsGroup.GET("/ls", s.handleFSList)          // List folder content
			fsGroup.POST("/upload", s.handleFSUpload)   // Upload file to specific folder
			fsGroup.DELETE("/delete", s.handleFSDelete) // Delete file or folder
			fsGroup.POST("/move", s.handleFSMove)       // Rename or Move
		}
	}

	return r
}

func (s *Server) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			return
		}

		// Format attendu : "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			return
		}

		tokenString := parts[1]

		// Appel gRPC au service Auth pour valider le token
		resp, err := s.auth.ValidateToken(c, &auth_pb.TokenRequest{Token: tokenString})
		if err != nil || !resp.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// On injecte le username dans le contexte pour les handlers suivants
		c.Set("username", resp.Username)
		c.Next()
	}
}

func (s *Server) handleRegisterInit(c *gin.Context) {
	var req struct {
		Username string
		Password string
		Email    string
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	_, err := s.auth.RegisterInit(c, &auth_pb.RegisterRequest{
		Username: req.Username, Password: req.Password, Email: req.Email,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OTP sent to email"})
}

func (s *Server) handleRegisterVerify(c *gin.Context) {
	var req struct {
		Email string
		Code  string
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}

	resp, err := s.auth.RegisterVerify(c, &auth_pb.OTPRequest{Email: req.Email, Code: req.Code})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleLoginInit(c *gin.Context) {
	var req struct {
		Username string
		Password string
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}

	resp, err := s.auth.LoginInit(c, &auth_pb.LoginRequest{Username: req.Username, Password: req.Password})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": resp.Message})
}

func (s *Server) handleLoginVerify(c *gin.Context) {
	var req struct {
		Email string
		Code  string
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}

	resp, err := s.auth.LoginVerify(c, &auth_pb.OTPRequest{Email: req.Email, Code: req.Code})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleCreateNode(c *gin.Context) {
	// Mapping JSON -> Proto
	var req struct {
		Name        string `json:"name"`
		MemoryMB    int64  `json:"memory_mb"`
		CpuShares   int64  `json:"cpu_shares"`
		StorageSize string `json:"storage_size"`
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}

	// Appel gRPC vers le Daemon Nexus
	resp, err := s.nexus.CreateNode(c, &pb.CreateNodeRequest{
		Name:        req.Name,
		MemoryMb:    req.MemoryMB,
		CpuShares:   req.CpuShares,
		StorageSize: req.StorageSize,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (s *Server) handleUpload(c *gin.Context) {
	// 1. Récupérer le fichier depuis la requête HTTP Multipart
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// 2. Sauvegarder temporairement le fichier sur le disque du serveur Gateway
	// Pour que le Daemon (qui tourne sur la même machine pour l'instant) puisse le lire.
	tempDir := "/tmp/nexus_uploads"
	os.MkdirAll(tempDir, 0755)
	tempPath := filepath.Join(tempDir, file.Filename)

	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save temp file"})
		return
	}

	// 3. Appeler le gRPC. On passe le chemin local du fichier.
	// Le Daemon va lire ce chemin et le traiter.
	resp, err := s.nexus.UploadFile(c, &pb.UploadFileRequest{
		LocalPath: tempPath,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleListFiles(c *gin.Context) {
	resp, err := s.nexus.ListFiles(c, &pb.Empty{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp.Files)
}

func (s *Server) handleDownload(c *gin.Context) {
	fileID := c.Param("id")

	// 1. Définir un endroit temporaire où le Daemon va déposer le fichier
	tempDest := filepath.Join("/tmp/nexus_downloads", fileID)
	os.MkdirAll("/tmp/nexus_downloads", 0755)

	// 2. Demander au Daemon de reconstruire le fichier à cet endroit
	_, err := s.nexus.DownloadFile(c, &pb.DownloadFileRequest{
		FileId:   fileID,
		DestPath: tempDest,
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found or reconstruction failed"})
		return
	}

	// 3. Servir le fichier via HTTP au client
	// Cela permet au navigateur de lancer le téléchargement
	c.File(tempDest)

}
func (s *Server) handleGetMetrics(c *gin.Context) {
	id := c.Param("id")
	resp, err := s.nexus.GetNodeMetrics(c, &pb.NodeMetricsRequest{NodeId: id})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, resp)
}
func (s *Server) handleRunLambda(c *gin.Context) {
	var req struct {
		Code    string
		Runtime string
	}
	if err := c.BindJSON(&req); err != nil {
		return
	}

	resp, err := s.nexus.RunLambda(c, &pb.LambdaRequest{Code: req.Code, Runtime: req.Runtime})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, resp)
}

func (s *Server) handleFSMkdir(c *gin.Context) {
	var req struct {
		Path string `json:"path"` // e.g., "/documents/projects"
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Retrieve username injected by AuthMiddleware
	username := c.GetString("username")

	_, err := s.nexus.FSMakeDir(c, &pb.FSRequest{
		Path:     req.Path,
		Username: username,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Directory created", "path": req.Path})
}

func (s *Server) handleFSList(c *gin.Context) {
	pathQuery := c.Query("path") // Get path from URL params
	if pathQuery == "" {
		pathQuery = "/" // Default to root
	}

	username := c.GetString("username")

	resp, err := s.nexus.FSList(c, &pb.FSRequest{
		Path:     pathQuery,
		Username: username,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Returns a list of file/folder objects
	c.JSON(http.StatusOK, resp.Items)
}

func (s *Server) handleFSUpload(c *gin.Context) {
	// 1. Get the target directory from the form data
	virtualPath := c.PostForm("path") // e.g., "/documents/images/logo.png"
	if virtualPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target path is required"})
		return
	}

	// 2. Get the file from the request
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// 3. Save to a temp folder (Gateway acts as a buffer)
	tempDir := "/tmp/nexus_fs_uploads"
	os.MkdirAll(tempDir, 0755)
	localTempPath := filepath.Join(tempDir, file.Filename)

	if err := c.SaveUploadedFile(file, localTempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to buffer file"})
		return
	}

	username := c.GetString("username")

	// 4. Call gRPC to process the logic
	// The Daemon will: 1. Chunk the file, 2. Store it, 3. Link it in the user's VFS
	_, err = s.nexus.FSUpload(c, &pb.FSUploadRequest{
		LocalPath:   localTempPath, // Physical path on server
		VirtualPath: virtualPath,   // Logical path in user's VFS
		Username:    username,
	})

	// Clean up temp file (optional, depends on if Daemon moves or copies)
	// os.Remove(localTempPath)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully", "path": virtualPath})
}

func (s *Server) handleFSDelete(c *gin.Context) {
	pathQuery := c.Query("path") // e.g., DELETE /api/fs/delete?path=/docs/old.txt
	if pathQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path required"})
		return
	}

	username := c.GetString("username")

	_, err := s.nexus.FSDelete(c, &pb.FSRequest{
		Path:     pathQuery,
		Username: username,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item deleted"})
}
func (s *Server) handleFSMove(c *gin.Context) {
	var req struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	username := c.GetString("username")

	_, err := s.nexus.FSMove(c, &pb.FSMoveRequest{
		OldPath:  req.OldPath,
		NewPath:  req.NewPath,
		Username: username,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Item moved/renamed"})
}
