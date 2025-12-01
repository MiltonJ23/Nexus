package identity

import (
	pb "Nexus/proto/auth"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var SecretKey = []byte("apotheose-secret-key-2025") // This is simply for the test , a proper implementation will be done using environment secrets

type User struct {
	gorm.Model
	Username string `gorm:"uniqueIndex"`
	Email    string `gorm:"uniqueIndex"`
	Password string
}
type PendingRegistration struct {
	Email     string `gorm:"primaryKey"`
	Username  string
	Password  string // Hashé (on le stocke temporairement en attendant l'OTP)
	OTP       string
	ExpiresAt time.Time
}
type LoginOTP struct {
	Email     string `gorm:"primaryKey"`
	OTP       string
	ExpiresAt time.Time
}

// --- Serveur ---

type AuthServer struct {
	pb.UnimplementedAuthServiceServer
	db *gorm.DB
}

func NewAuthServer() (*AuthServer, error) {
	db, err := gorm.Open(sqlite.Open("/var/lib/nexus/auth.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// Création automatique des tables
	db.AutoMigrate(&User{}, &PendingRegistration{}, &LoginOTP{})
	return &AuthServer{db: db}, nil
}

// --- 1. REGISTER FLOW ---

func (s *AuthServer) RegisterInit(ctx context.Context, req *pb.RegisterRequest) (*pb.GenericResponse, error) {
	var exists int64
	s.db.Model(&User{}).Where("username = ? OR email = ?", req.Username, req.Email).Count(&exists)
	if exists > 0 {
		return nil, errors.New("username or email already exists")
	}

	// Hasher le mot de passe
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	otp := GenerateOTP()

	// Sauvegarder dans la table "Pending" (En attente de validation)
	reg := PendingRegistration{
		Email: req.Email, Username: req.Username, Password: string(hashedPwd),
		OTP: otp, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	s.db.Save(&reg) // Upsert (écrase si existe déjà pour cet email)

	// Goroutine pour ne pas bloquer l'API !
	go func() {
		if err := SendOTPEmail(req.Email, otp); err != nil {
			fmt.Printf("-> Failed to send register OTP: %v\n", err)
		}
	}()

	return &pb.GenericResponse{Message: "OTP sent to email"}, nil
}

func (s *AuthServer) RegisterVerify(ctx context.Context, req *pb.OTPRequest) (*pb.TokenResponse, error) {
	var pending PendingRegistration
	// Chercher la demande en attente
	if err := s.db.Where("email = ? AND otp = ?", req.Email, req.Code).First(&pending).Error; err != nil {
		return nil, errors.New("invalid or expired OTP")
	}

	if time.Now().After(pending.ExpiresAt) {
		return nil, errors.New("OTP expired")
	}

	// OTP valide : Créer le vrai User
	user := User{Username: pending.Username, Email: pending.Email, Password: pending.Password}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	// Nettoyer
	s.db.Delete(&pending)

	// Générer JWT direct
	token, _ := generateJWT(user.Username)
	return &pb.TokenResponse{Token: token, Username: user.Username}, nil
}

// --- 2. LOGIN FLOW ---

func (s *AuthServer) LoginInit(ctx context.Context, req *pb.LoginRequest) (*pb.GenericResponse, error) {
	var user User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Vérifier mot de passe
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Mot de passe OK -> Générer OTP
	otp := GenerateOTP()
	loginOtp := LoginOTP{
		Email: user.Email, OTP: otp, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	s.db.Save(&loginOtp)

	// Envoyer mail en background
	go func() {
		if err := SendOTPEmail(user.Email, otp); err != nil {
			fmt.Printf("-> Failed to send login OTP: %v\n", err)
		}
	}()

	// On retourne l'email (partiellement masqué idéalement) pour que le front sache où l'OTP a été envoyé
	return &pb.GenericResponse{Message: "OTP sent to " + user.Email}, nil
}

func (s *AuthServer) LoginVerify(ctx context.Context, req *pb.OTPRequest) (*pb.TokenResponse, error) {
	var loginOtp LoginOTP
	if err := s.db.Where("email = ? AND otp = ?", req.Email, req.Code).First(&loginOtp).Error; err != nil {
		return nil, errors.New("invalid OTP")
	}

	if time.Now().After(loginOtp.ExpiresAt) {
		return nil, errors.New("OTP expired")
	}

	// Retrouver le username via l'email
	var user User
	s.db.Where("email = ?", req.Email).First(&user)

	// Nettoyer
	s.db.Delete(&loginOtp)

	// Token
	token, _ := generateJWT(user.Username)
	return &pb.TokenResponse{Token: token, Username: user.Username}, nil
}

// --- Helpers ---

func generateJWT(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(SecretKey)
}

func (s *AuthServer) ValidateToken(ctx context.Context, req *pb.TokenRequest) (*pb.ValidationResponse, error) {
	token, err := jwt.Parse(req.Token, func(token *jwt.Token) (interface{}, error) {
		return SecretKey, nil
	})
	if err != nil || !token.Valid {
		return &pb.ValidationResponse{Valid: false}, nil
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	return &pb.ValidationResponse{Valid: true, Username: fmt.Sprint(claims["sub"])}, nil
}
