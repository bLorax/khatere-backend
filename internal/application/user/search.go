package user

import (
	"context"

	domainuser "yadegar/internal/domain/user"
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

func (uc *SearchUsersUseCase) Execute(ctx context.Context, query string) ([]domainuser.User, error) {
	if len(query) < minSearchQueryLen {
		return []domainuser.User{}, nil
	}
	return uc.repo.Search(ctx, query, searchResultLimit)
}

// GetUserUseCase returns one user by ID.
type GetUserUseCase struct {
	repo domainuser.Repository
}

func NewGetUserUseCase(repo domainuser.Repository) *GetUserUseCase {
	return &GetUserUseCase{repo: repo}
}

func (uc *GetUserUseCase) Execute(ctx context.Context, id string) (*domainuser.User, error) {
	return uc.repo.Get(ctx, id)
}
