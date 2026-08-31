package user

import (
	"context"
	domainuser "yadegar/internal/domain/user"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"yadegar/internal/telemetry"
)

const minSearchQueryLen = 2
const searchResultLimit = 20

// SearchUsersUseCase looks up users by a partial username.
type SearchUsersUseCase struct {
	repo domainuser.Repository
}

func NewSearchUsersUseCase(repo domainuser.Repository) *SearchUsersUseCase {
	return &SearchUsersUseCase{repo: repo}
}

// GetUserUseCase returns one user by ID.
type GetUserUseCase struct {
	repo domainuser.Repository
}

func NewGetUserUseCase(repo domainuser.Repository) *GetUserUseCase {
	return &GetUserUseCase{repo: repo}
}

func (uc *SearchUsersUseCase) Execute(ctx context.Context, query string) ([]domainuser.User, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "SearchUsersUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("search.query", query))

	if len(query) < minSearchQueryLen {
		return []domainuser.User{}, nil
	}

	results, err := uc.repo.Search(ctx, query, searchResultLimit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return results, nil
}

func (uc *GetUserUseCase) Execute(ctx context.Context, id string) (*domainuser.User, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "GetUserUseCase.Execute")
	defer span.End()
	span.SetAttributes(attribute.String("user.id", id))

	u, err := uc.repo.Get(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return u, nil
}
