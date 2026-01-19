package ebuild

import (
	"context"

	"github.com/grpmsoft/grpm/internal/distfile"
	"github.com/grpmsoft/grpm/internal/pkg"
)

// srcURIEvaluatorAdapter implements distfile.SrcURIEvaluator using
// the ebuild.EvaluateSrcURI function.
type srcURIEvaluatorAdapter struct{}

// NewSrcURIEvaluator creates an adapter that implements distfile.SrcURIEvaluator.
func NewSrcURIEvaluator() distfile.SrcURIEvaluator {
	return &srcURIEvaluatorAdapter{}
}

// EvaluateSrcURI implements distfile.SrcURIEvaluator.
func (a *srcURIEvaluatorAdapter) EvaluateSrcURI(
	ctx context.Context,
	ebuildPath, repoPath string,
	pkgInfo *pkg.Package,
) (string, error) {
	return EvaluateSrcURI(ctx, ebuildPath, repoPath, pkgInfo)
}
