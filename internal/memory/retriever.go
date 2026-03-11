package memory

import "context"

// Retriever wraps a ChromaClient and Embedder to provide semantic memory
// retrieval. It implements the runner.MemoryRetriever interface.
type Retriever struct {
	Client   *ChromaClient
	Embedder Embedder
}

// NewRetriever creates a Retriever from a ChromaClient and Embedder.
// Returns nil if either dependency is nil.
func NewRetriever(client *ChromaClient, embedder Embedder) *Retriever {
	if client == nil || embedder == nil {
		return nil
	}
	return &Retriever{Client: client, Embedder: embedder}
}

// RetrieveContext queries all memory collections and returns formatted markdown
// for prompt injection. Delegates to the package-level RetrieveContext function.
func (r *Retriever) RetrieveContext(ctx context.Context, storyTitle, storyDescription string, acceptanceCriteria []string, opts RetrievalOptions) (string, error) {
	return RetrieveContext(ctx, r.Client, r.Embedder, storyTitle, storyDescription, acceptanceCriteria, opts)
}
