package adapters

import "context"

// ImageRequest carries everything an image adapter needs to generate or
// revise a portrait/illustration.
//
//   - ReferenceImage nil: first generation — Prompt should be a full
//     descriptive prompt built from the source data (e.g. entity attributes).
//   - ReferenceImage set: a revision pass — Prompt is a short edit
//     instruction ("make the hair silver") applied against the reference.
type ImageRequest struct {
	Prompt         string
	ReferenceImage []byte // PNG bytes of the prior draft, or nil
}

// ImageResult is the generated image returned by an ImageAdapter.
type ImageResult struct {
	ImageBytes  []byte
	ContentType string // "image/png"
}

// ImageAdapter is the common interface all image-generation backends must
// satisfy. Kept separate from Adapter (text) since image providers are a
// disjoint set of vendors/capabilities from chat providers.
type ImageAdapter interface {
	// GenerateImage produces a new image from req. When req.ReferenceImage is
	// set, the returned image should be a revision of that reference guided
	// by req.Prompt rather than an unrelated new image.
	GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, Usage, error)

	// Provider returns the canonical provider name ("openai").
	Provider() string
}
