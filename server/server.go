package server

import (
	"context"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"
	"github.com/rs/zerolog/log"
	"github.com/snyk/snyk-lsp/code"
	"github.com/snyk/snyk-lsp/diagnostics"
	"github.com/snyk/snyk-lsp/lsp"
	sglsp "github.com/sourcegraph/go-lsp"
	"os"
)

var (
	clientParams sglsp.InitializeParams
)

func Start() {
	var srv *jrpc2.Server
	var snykCodeBackendService code.SnykCodeBackendService

	lspHandlers := handler.Map{
		"initialize":                     InitializeHandler(&snykCodeBackendService),
		"textDocument/didOpen":           TextDocumentDidOpenHandler(&srv, &snykCodeBackendService),
		"textDocument/didChange":         TextDocumentDidChangeHandler(),
		"textDocument/didClose":          TextDocumentDidCloseHandler(),
		"textDocument/didSave":           TextDocumentDidSaveHandler(&srv, &snykCodeBackendService),
		"textDocument/willSave":          TextDocumentWillSaveHandler(),
		"textDocument/willSaveWaitUntil": TextDocumentWillSaveWaitUntilHandler(),
		"shutdown":                       Shutdown(),
		"exit":                           Exit(&srv),
		"textDocument/codeLens":          TextDocumentCodeLens(),
		//"codeLens/resolve":               codeLensResolve(&server),
	}

	srv = jrpc2.NewServer(lspHandlers, &jrpc2.ServerOptions{
		AllowPush: true,
	})

	log.Info().Msg("Starting up...")
	srv = srv.Start(channel.Header("")(os.Stdin, os.Stdout))

	err := srv.Wait()
	log.Err(err).Msg("Exiting...")
}

func Shutdown() jrpc2.Handler {
	return handler.New(func(ctx context.Context) (interface{}, error) {
		log.Info().Str("method", "Shutdown").Msg("RECEIVING")
		log.Info().Str("method", "Shutdown").Msg("SENDING")
		return nil, nil
	})
}

func Exit(srv **jrpc2.Server) jrpc2.Handler {
	return handler.New(func(ctx context.Context) (interface{}, error) {
		log.Info().Str("method", "Exit").Msg("RECEIVING")
		log.Info().Msg("Stopping server...")
		(*srv).Stop()
		return nil, nil
	})
}

func TextDocumentCodeLens() handler.Func {
	return handler.New(func(ctx context.Context, params sglsp.CodeLensParams) (interface{}, error) {
		log.Info().Str("method", "TextDocumentCodeLens").Interface("params", params).Msg("RECEIVING")

		codeLenses, err := diagnostics.GetCodeLenses(params.TextDocument.URI)
		if err != nil {
			log.Err(err).Str("method", "TextDocumentCodeLens")
		}

		log.Info().Str("method", "TextDocumentCodeLens").Interface("response", codeLenses).Msg("SENDING")
		return codeLenses, err
	})
}

func TextDocumentDidChangeHandler() handler.Func {
	return handler.New(func(ctx context.Context, params sglsp.DidChangeTextDocumentParams) (interface{}, error) {
		log.Info().Str("method", "TextDocumentDidChangeHandler").Interface("params", params).Msg("RECEIVING")
		diagnostics.UpdateDocument(params.TextDocument.URI, params.ContentChanges)
		return nil, nil
	})
}

func PublishDiagnostics(ctx context.Context, uri sglsp.DocumentURI, srv **jrpc2.Server, backendService code.BackendService) {
	diags, err := diagnostics.GetDiagnostics(uri, backendService)
	logError(err, "PublishDiagnostics")
	if diags != nil {
		diagnosticsParams := lsp.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diags,
		}
		log.Info().Str("method", "PublishDiagnostics").Interface("params", diagnosticsParams).Msg("SENDING")
		err := (*srv).Notify(ctx, "textDocument/publishDiagnostics", diagnosticsParams)
		logError(err, "PublishDiagnostics")
	}
}

func logError(err error, method string) {
	if err != nil {
		log.Err(err).Str("method", method)
	}
}

func TextDocumentDidOpenHandler(srv **jrpc2.Server, backendService code.BackendService) handler.Func {
	return handler.New(func(ctx context.Context, params sglsp.DidOpenTextDocumentParams) (interface{}, error) {
		log.Info().Str("method", "TextDocumentDidOpenHandler").Interface("params", params).Msg("RECEIVING")
		diagnostics.RegisterDocument(params.TextDocument)
		PublishDiagnostics(ctx, params.TextDocument.URI, srv, backendService)
		return nil, nil
	})
}

func TextDocumentDidSaveHandler(srv **jrpc2.Server, backendService code.BackendService) handler.Func {
	return handler.New(func(ctx context.Context, params sglsp.DidSaveTextDocumentParams) (interface{}, error) {
		log.Info().Str("method", "TextDocumentDidSaveHandler").Interface("params", params).Msg("RECEIVING")
		// clear cache when saving and get fresh diagnostics
		diagnostics.ClearDiagnosticsCache(params.TextDocument.URI)
		diagnostics.ClearLenses(params.TextDocument.URI)
		PublishDiagnostics(ctx, params.TextDocument.URI, srv, backendService)
		return nil, nil
	})
}

func TextDocumentWillSaveHandler() handler.Func {
	return handler.New(func(ctx context.Context, params lsp.WillSaveTextDocumentParams) (interface{}, error) {
		log.Info().Str("method", "TextDocumentWillSaveHandler").Interface("params", params).Msg("RECEIVING")
		return nil, nil
	})
}

func TextDocumentWillSaveWaitUntilHandler() handler.Func {
	return handler.New(func(ctx context.Context, params lsp.WillSaveTextDocumentParams) (interface{}, error) {
		log.Info().Str("method", "TextDocumentWillSaveWaitUntilHandler").Interface("params", params).Msg("RECEIVING")
		return nil, nil
	})
}

func TextDocumentDidCloseHandler() handler.Func {
	return handler.New(func(ctx context.Context, params sglsp.DidCloseTextDocumentParams) (interface{}, error) {
		log.Info().Str("method", "TextDocumentDidCloseHandler").Interface("params", params).Msg("RECEIVING")
		diagnostics.UnRegisterDocument(params.TextDocument.URI)
		return nil, nil
	})
}

func InitializeHandler(snykCodeBackend code.BackendService) handler.Func {
	return handler.New(func(ctx context.Context, params sglsp.InitializeParams) (interface{}, error) {
		log.Info().Str("method", "InitializeHandler").Interface("params", params).Msg("RECEIVING")
		clientParams = params
		go diagnostics.GetDiagnostics(clientParams.RootURI, snykCodeBackend)
		return lsp.InitializeResult{
			Capabilities: lsp.ServerCapabilities{
				TextDocumentSync: &sglsp.TextDocumentSyncOptionsOrKind{
					Options: &sglsp.TextDocumentSyncOptions{
						OpenClose:         true,
						Change:            sglsp.TDSKFull,
						WillSave:          true,
						WillSaveWaitUntil: true,
						Save:              &sglsp.SaveOptions{IncludeText: true},
					},
				},
				CodeLensProvider: &sglsp.CodeLensOptions{ResolveProvider: true},
				WorkspaceFoldersServerCapabilities: &lsp.WorkspaceFoldersServerCapabilities{
					Supported:           true,
					ChangeNotifications: "snyk-lsp",
				},
			},
		}, nil
	})
}
