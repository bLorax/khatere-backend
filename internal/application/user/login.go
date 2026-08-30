package user

import (
	"context"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	domainuser "yadegar/internal/domain/user"
)

// LoginUseCase checks credentials and issues a JWT.
type LoginUseCase struct {
	repo domainuser.Repository
}

func NewLoginUseCase(repo domainuser.Repository) *LoginUseCase {
	return &LoginUseCase{repo: repo}
}

// LoginInput holds the raw data for a login request.
type LoginInput struct {
	Identifier string
	Password   string
}

// LoginOutput holds the result of a successful login.
type LoginOutput struct {
	Token    string
	ID       string
	Username string
}

// Execute runs the login use case.
func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (*LoginOutput, error) {
	u, err := uc.repo.FindByIdentifier(ctx, in.Identifier)
	if err != nil {
		// Return one generic error for "not found" and "wrong password".
		// This rule stops an attacker from learning which identifiers exist.
		return nil, domainuser.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return nil, domainuser.ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": u.ID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})

	// NOTE: this step still reads the JWT secret from the environment directly.
	// A later step can move this read into a config port. This step keeps the
	// change small and focused on layering.
	signed, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return nil, err
	}

	return &LoginOutput{Token: signed, ID: u.ID, Username: u.Username}, nil
}
