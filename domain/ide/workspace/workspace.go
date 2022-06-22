package workspace

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
	sglsp "github.com/sourcegraph/go-lsp"

	"github.com/snyk/snyk-ls/di"
	"github.com/snyk/snyk-ls/internal/notification"
	"github.com/snyk/snyk-ls/internal/preconditions"
	"github.com/snyk/snyk-ls/internal/uri"
	"github.com/snyk/snyk-ls/lsp"
)

// TODO test AddFolder
func (w *Workspace) AddFolder(f *Folder) {
	w.workspaceFolders = append(w.workspaceFolders, f)
}

// TODO test GetFolder
func (w *Workspace) GetFolder(path string) (folder *Folder) {
	for _, folder := range w.workspaceFolders {
		if folder.Contains(path) {
			return folder
		}
	}
	return &Folder{}
}

func (w *Workspace) GetDiagnostics(ctx context.Context, path string) []lsp.Diagnostic {
	// serve from cache
	method := "Workspace.GetDiagnostics"
	s := di.Instrumentor().NewTransaction(ctx, method, method)
	defer di.Instrumentor().Finish(s)

	folder := w.GetFolder(path)

	diagnosticSlice := folder.documentDiagnosticsFromCache(path)
	if len(diagnosticSlice) > 0 {
		log.Info().Str("method", method).Msgf("Cached: Diagnostics for %s", path)
		return diagnosticSlice
	}

	folder.FetchAllRegisteredDocumentDiagnostics(s.Context(), path, lsp.ScanLevelFile)
	return folder.documentDiagnosticsFromCache(path)
}

func (w *Workspace) Scan(ctx context.Context) {
	method := "domain.ide.Workspace.Scan"
	s := di.Instrumentor().NewTransaction(ctx, method, method)
	defer di.Instrumentor().Finish(s)

	preconditions.EnsureReadyForAnalysisAndWait(ctx)
	notification.Send(sglsp.ShowMessageParams{Type: sglsp.Info, Message: "Workspace scan started"})
	defer notification.Send(sglsp.ShowMessageParams{Type: sglsp.Info, Message: "Workspace scan completed"})

	var wg sync.WaitGroup
	for _, folder := range w.workspaceFolders {
		wg.Add(1)
		go folder.Scan(s.Context(), &wg)
	}

	wg.Wait()
	log.Info().Str("method", "Workspace").
		Msg("Workspace scan completed")
}

// todo test
func (w Workspace) getWorkspaceFolderOfDocument(documentPath string) (folder *Folder) {
	for _, folder := range w.workspaceFolders {
		if uri.FolderContains(folder.path, documentPath) {
			return folder
		}
	}
	return &Folder{}
}
