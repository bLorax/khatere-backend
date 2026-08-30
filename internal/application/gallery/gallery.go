// Package gallery holds the use case for the Gallery read-model.
package gallery

import (
	"context"

	domaingallery "yadegar/internal/domain/gallery"
)

type ListGalleryUseCase struct {
	repo domaingallery.Repository
}

func NewListGalleryUseCase(repo domaingallery.Repository) *ListGalleryUseCase {
	return &ListGalleryUseCase{repo: repo}
}

func (uc *ListGalleryUseCase) Execute(ctx context.Context, userID string) ([]domaingallery.Event, error) {
	return uc.repo.ListForUser(ctx, userID)
}
