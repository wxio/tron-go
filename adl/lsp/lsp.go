package lsp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/golangq/q"
	"golang.org/x/tools/jsonrpc2"
	"golang.org/x/tools/lsp/protocol"
)

type server struct {
	version    string
	adlcPath   string
	tempDir    string
	extConfig  TronExtCfg
	langCfg    TronLangCfg
	client     protocol.Client
	conn       *jsonrpc2.Conn
	initParams *protocol.InitializeParams
	cancel     context.CancelFunc
	// tcpConn    net.Conn
	fileCache filecache
	astCache  astcache
}

func (svr *server) Initialize(ctx context.Context, req *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	q.Q(svr.adlcPath)
	tempDir, err := ioutil.TempDir("", "adl-lsp")
	if err != nil {
		q.Q(err)
		return nil, err
	}
	svr.tempDir = tempDir
	q.Q(req)
	svr.fileCache = newCache()
	svr.astCache = newAstCache()
	svr.initParams = req
	// type ServerCapabilities struct {
	// 	InnerServerCapabilities
	// 	ImplementationServerCapabilities
	// 	TypeDefinitionServerCapabilities
	// 	WorkspaceFoldersServerCapabilities
	// 	ColorServerCapabilities
	// 	FoldingRangeServerCapabilities
	// 	DeclarationServerCapabilities
	// 	SelectionRangeServerCapabilities
	// }
	// q.Q(spew.Sdump(req))
	textDocumentSyncKind := protocol.Full
	ret := &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			InnerServerCapabilities: protocol.InnerServerCapabilities{
				CodeActionProvider: true,
				CompletionProvider: &protocol.CompletionOptions{
					TriggerCharacters: []string{"."},
				},
				DefinitionProvider:              true,
				DocumentFormattingProvider:      true,
				DocumentRangeFormattingProvider: true,
				DocumentSymbolProvider:          true,
				ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
					Commands: []string{"tron.compile", "tron.browse"},
				},
				HoverProvider:             true,
				DocumentHighlightProvider: true,
				SignatureHelpProvider: &protocol.SignatureHelpOptions{
					TriggerCharacters: []string{"(", ","},
				},
				TextDocumentSync: &protocol.TextDocumentSyncOptions{
					Change:    textDocumentSyncKind,
					OpenClose: true,
				},
			},
			TypeDefinitionServerCapabilities: protocol.TypeDefinitionServerCapabilities{
				TypeDefinitionProvider: true,
			},
			ImplementationServerCapabilities: protocol.ImplementationServerCapabilities{
				ImplementationProvider: true,
			},
			WorkspaceFoldersServerCapabilities: protocol.WorkspaceFoldersServerCapabilities{
				Workspace: &struct {
					WorkspaceFolders *struct {
						Supported           bool   `json:"supported,omitempty"`
						ChangeNotifications string `json:"changeNotifications,omitempty"`
					} `json:"workspaceFolders,omitempty"`
				}{
					WorkspaceFolders: &struct {
						Supported           bool   `json:"supported,omitempty"`
						ChangeNotifications string `json:"changeNotifications,omitempty"`
					}{
						Supported:           true,
						ChangeNotifications: "true",
					},
				},
			},
			DeclarationServerCapabilities: protocol.DeclarationServerCapabilities{
				DeclarationProvider: true,
			},
		},
	}

	// ret := &protocol.InitializeResult{
	// 	Capabilities: protocol.ServerCapabilities{},
	// 	Custom:       make(map[string]interface{}),
	// }
	return ret, nil
}
func (svr *server) Initialized(ctx context.Context, req *protocol.InitializedParams) error {
	q.Q(req)

	// go func() {
	// 	for {
	// 		select {
	// 		case <-time.After(10 * time.Second):
	// 			svr.conn.Notify(ctx, "window/logMessage", protocol.LogMessageParams{
	// 				Message: "TRON LSP (version" + svr.version + ")",
	// 				Type:    protocol.Info,
	// 			})
	// 		case <-ctx.Done():
	// 			q.Q("log message exit")
	// 			return
	// 		}
	// 	}
	// }()

	defer func() {
		svr.conn.Notify(ctx, "window/logMessage", protocol.LogMessageParams{
			Message: "TRON LSP (version " + svr.version + ")",
			Type:    protocol.Info,
		})
		err := svr.client.ShowMessage(ctx, &protocol.ShowMessageParams{
			Message: "Started TRON LSP (version " + svr.version + ")",
			Type:    protocol.Info,
		})
		if err != nil {
			q.Q(err)
			// return err // TODO what does returning an error do
		}
	}()

	// svr.initParams.Capabilities
	// Check if the client supports configuration messages.
	if x, ok := svr.initParams.Capabilities["workspace"].(map[string]interface{}); ok {
		if x, ok := x["configuration"].(bool); ok {
			q.Q("configurationSupported", x)
		}
		if x, ok := x["didChangeConfiguration"].(map[string]interface{}); ok {
			if x, ok := x["dynamicRegistration"].(bool); ok {
				q.Q("dynamicConfigurationSupported", x)
			}
		}
	}

	// err := svr.client.RegisterCapability(ctx, &protocol.RegistrationParams{
	// 	Registrations: []protocol.Registration{{
	// 		ID:     "1234567890",
	// 		Method: "workspace/didChangeConfiguration",
	// 	}},
	// })
	// if err != nil {
	// 	q.Q(err)
	// 	// return err // TODO what does returning an error do
	// }

	ctx2 := context.Background()
	wfs, err := svr.client.WorkspaceFolders(ctx2)
	if err != nil {
		q.Q(err)
		// return err // TODO what does returning an error do
	}

	if len(wfs) == 0 {
		q.Q("empty WorkspaceFolder root:", svr.initParams.RootURI)
		if svr.initParams.RootURI != "" {
			wfs = []protocol.WorkspaceFolder{{
				URI:  svr.initParams.RootURI,
				Name: path.Base(svr.initParams.RootURI),
			}}
		} else {
			// no folders and no root, single file mode
			//TODO(iancottrell): not sure how to do single file mode yet
			//issue: golang.org/issue/31168
			q.Q(fmt.Errorf("single file mode not supported yet"))
		}
	}

	var items []protocol.ConfigurationItem
	for _, wf := range wfs {
		// q.Q(wf)
		items = append(items, []protocol.ConfigurationItem{
			{ScopeURI: wf.URI, Section: "tron"},
			{ScopeURI: wf.URI, Section: "[tron]"},
		}...,
		)
	}
	var result []TronCfg
	if err := svr.conn.Call(ctx2, "workspace/configuration", &protocol.ConfigurationParams{Items: items}, &result); err != nil {
		q.Q(err)
		return err
	}
	for _, x := range result {
		if x.TronExtCfg != nil {
			svr.extConfig = *x.TronExtCfg
		}
		if x.TronLangCfg != nil {
			svr.langCfg = *x.TronLangCfg
		}
	}
	return nil
}
func (svr *server) Shutdown(context.Context) error {
	q.Q("shutdown")
	svr.cancel()
	os.Exit(0)
	// svr.tcpConn.Close()
	return nil
}
func (svr *server) Exit(context.Context) error {
	q.Q("exit")
	return nil
}
func (svr *server) DidChangeWorkspaceFolders(ctx context.Context, req *protocol.DidChangeWorkspaceFoldersParams) error {
	q.Q(req)
	return nil
}
func (svr *server) DidChangeConfiguration(ctx context.Context, req *protocol.DidChangeConfigurationParams) error {
	q.Q(req)
	return nil
}
func (svr *server) DidChangeWatchedFiles(ctx context.Context, req *protocol.DidChangeWatchedFilesParams) error {
	q.Q(req)
	return nil
}
func (svr *server) Symbols(ctx context.Context, req *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) ExecuteCommand(ctx context.Context, req *protocol.ExecuteCommandParams) (interface{}, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) DidOpen(ctx context.Context, req *protocol.DidOpenTextDocumentParams) error {
	q.Q(req)
	if strings.HasSuffix(req.TextDocument.URI, ".mod") {
		return nil
	}
	svr.fileCache.put(req.TextDocument.URI, req.TextDocument.Text)
	svr.diag(ctx, req.TextDocument.URI, req.TextDocument.Text)
	return nil
}
func (svr *server) DidChange(ctx context.Context, req *protocol.DidChangeTextDocumentParams) error {
	q.Q(req)
	if strings.HasSuffix(req.TextDocument.URI, ".mod") {
		return nil
	}
	if len(req.ContentChanges) < 1 {
		q.Q("no change")
		return nil
	}
	change := req.ContentChanges[0]
	svr.fileCache.put(req.TextDocument.URI, change.Text)
	svr.diag(ctx, req.TextDocument.URI, change.Text)
	return nil
}
func (svr *server) WillSave(ctx context.Context, req *protocol.WillSaveTextDocumentParams) error {
	q.Q(req)
	return nil
}
func (svr *server) WillSaveWaitUntil(ctx context.Context, req *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) DidSave(ctx context.Context, req *protocol.DidSaveTextDocumentParams) error {
	q.Q(req)
	return nil
}
func (svr *server) DidClose(ctx context.Context, req *protocol.DidCloseTextDocumentParams) error {
	q.Q(req)
	if strings.HasSuffix(req.TextDocument.URI, ".mod") {
		return nil
	}
	svr.fileCache.delete(req.TextDocument.URI)
	svr.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		Diagnostics: []protocol.Diagnostic{},
		URI:         req.TextDocument.URI,
	})
	return nil
}
func (svr *server) Completion(ctx context.Context, req *protocol.CompletionParams) (*protocol.CompletionList, error) {
	q.Q(req)
	return svr.collect(ctx, req)
}

func (svr *server) CompletionResolve(ctx context.Context, req *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) Hover(ctx context.Context, req *protocol.TextDocumentPositionParams) (*protocol.Hover, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) SignatureHelp(ctx context.Context, req *protocol.TextDocumentPositionParams) (*protocol.SignatureHelp, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) Definition(ctx context.Context, req *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) TypeDefinition(ctx context.Context, req *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) Implementation(ctx context.Context, req *protocol.TextDocumentPositionParams) ([]protocol.Location, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) References(ctx context.Context, req *protocol.ReferenceParams) ([]protocol.Location, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) DocumentHighlight(ctx context.Context, req *protocol.TextDocumentPositionParams) ([]protocol.DocumentHighlight, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) DocumentSymbol(ctx context.Context, req *protocol.DocumentSymbolParams) ([]protocol.DocumentSymbol, error) {
	defer func() {
		if r := recover(); r != nil {
			qstack()
		}
	}()
	q.Q(req)
	txt, err := svr.fileCache.get(req.TextDocument.URI)
	if err != nil {
		return []protocol.DocumentSymbol{}, nil
	}
	dsa, err := buildDocSym(txt, req.TextDocument.URI)
	if err != nil {
		q.Q(err)
		return []protocol.DocumentSymbol{}, nil
	}
	return dsa, nil
}

func (svr *server) CodeAction(ctx context.Context, req *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) CodeLens(ctx context.Context, req *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) CodeLensResolve(ctx context.Context, req *protocol.CodeLens) (*protocol.CodeLens, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) DocumentLink(ctx context.Context, req *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) DocumentLinkResolve(ctx context.Context, req *protocol.DocumentLink) (*protocol.DocumentLink, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) DocumentColor(ctx context.Context, req *protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) ColorPresentation(ctx context.Context, req *protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) Formatting(ctx context.Context, req *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) RangeFormatting(ctx context.Context, req *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) OnTypeFormatting(ctx context.Context, req *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) Rename(ctx context.Context, req *protocol.RenameParams) ([]protocol.WorkspaceEdit, error) {
	q.Q(req)
	return nil, nil
}
func (svr *server) FoldingRanges(ctx context.Context, req *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	q.Q(req)
	return nil, nil
}
