// Package user holds the use cases for the user domain. A use case holds
// one application action. A use case calls the domain layer. A use case
// calls a port. A use case does not know which adapter serves the port.
package user

import (
	"context"

	domainuser "yadegar/internal/domain/user"
	"yadegar/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/crypto/bcrypt"
)

// RegisterUseCase creates a new user account.
type RegisterUseCase struct {
	repo domainuser.Repository
}

// NewRegisterUseCase builds a RegisterUseCase. Pass any type that meets the
// domainuser.Repository port, for example a Postgres adapter or a fake for tests.
func NewRegisterUseCase(repo domainuser.Repository) *RegisterUseCase {
	return &RegisterUseCase{repo: repo}
}

// RegisterInput holds the raw data for a register request.
type RegisterInput struct {
	Username string
	Email    string
	Password string
}

func (uc *RegisterUseCase) Execute(ctx context.Context, in RegisterInput) (*domainuser.User, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "RegisterUseCase.Execute")
	defer span.End()

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	u := &domainuser.User{
		ID:           uuid.New().String(),
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: string(hash),
	}

	if err := uc.repo.Create(ctx, u); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return u, nil
}
