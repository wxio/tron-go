package lsp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/golangq/q"
	antlr "github.com/wxio/goantlr"
	"github.com/wxio/tron-go/adl"
	"golang.org/x/tools/jsonrpc2"
	"golang.org/x/tools/lsp/protocol"
)

type server struct {
	client     protocol.Client
	conn       *jsonrpc2.Conn
	initParams *protocol.InitializeParams
	cancel     context.CancelFunc
	// tcpConn    net.Conn
}

func (svr *server) Initialize(ctx context.Context, req *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	q.Q(req)
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
	q.Q(spew.Sdump(req))
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

	go func() {
		for {
			select {
			case <-time.After(10 * time.Second):
				svr.conn.Notify(ctx, "window/logMessage", protocol.LogMessageParams{
					Message: fmt.Sprintf("hello from tronlsp %v", time.Now()),
					Type:    protocol.Info,
				})
			case <-ctx.Done():
				q.Q("log message exit")
				return
			}
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
		q.Q(wf)
		items = append(items, []protocol.ConfigurationItem{
			{ScopeURI: wf.URI, Section: "tron"},
			{ScopeURI: wf.URI, Section: "[tron]"},
		}...,
		)
	}
	result, err := svr.client.Configuration(ctx2, &protocol.ConfigurationParams{Items: items})
	if err != nil {
		q.Q(err)
		// return err // TODO what does returning an error do
	}
	q.Q("config", result)

	// for _, view := range svr.views {
	// 	config, err := svr.client.Configuration(ctx, &protocol.ConfigurationParams{
	// 		Items: []protocol.ConfigurationItem{{
	// 			ScopeURI: protocol.NewURI(view.Folder),
	// 			Section:  "gopls",
	// 		}},
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if err := s.processConfig(view, config[0]); err != nil {
	// 		return err
	// 	}
	// }

	err = svr.client.ShowMessage(ctx, &protocol.ShowMessageParams{
		Message: "hello from tronlsp",
		Type:    protocol.Info,
	})
	if err != nil {
		q.Q(err)
		// return err // TODO what does returning an error do
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
	svr.diag(ctx, req.TextDocument.URI, req.TextDocument.Text)
	return nil
}
func (svr *server) DidChange(ctx context.Context, req *protocol.DidChangeTextDocumentParams) error {
	q.Q(req)
	if len(req.ContentChanges) < 1 {
		q.Q("no change")
		return nil
	}
	change := req.ContentChanges[0]
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
	return nil
}
func (svr *server) Completion(ctx context.Context, req *protocol.CompletionParams) (*protocol.CompletionList, error) {
	q.Q(req)
	cl := protocol.CompletionList{
		IsIncomplete: false,
		Items: []protocol.CompletionItem{
			{
				Label:         "test",
				Kind:          protocol.TextCompletion,
				Detail:        "This is a test",
				Documentation: "this is the docs",
			},
			{
				Label:         "test2",
				Kind:          protocol.TextCompletion,
				Detail:        "This is a test2",
				Documentation: "this is the docs2",
			},
		},
	}
	return &cl, nil
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
	fname := req.TextDocument.URI
	if strings.HasPrefix(fname, "file://") {
		fname = fname[len("file://"):]
	}
	by, err := ioutil.ReadFile(fname)
	if err != nil {
		q.Q(err)
		return nil, nil
	}

	tr, atr, bl, ts, err1 := adl.BuildAdlAST(string(by))
	_, _, _ = atr, bl, ts
	if err1.Error() != nil {
		q.Q(err1.Error())
		return nil, nil
	}
	if tr != nil {
		dsa := make([]protocol.DocumentSymbol, 0)
		ds := &docSym{
			dsa:       &dsa,
			lexStream: ts,
			furi:      fname,
		}
		err := adl.VisitAdlWo(tr, ds)
		if err.Error() != nil {
			q.Q(err.Error())
			return nil, nil
		}
		// x, _ := json.MarshalIndent(ds.dsa, "", "  ")
		// q.Q(string(x))
		return *ds.dsa, nil
	}
	return nil, nil
}

type errColl struct {
	errs []antlr.ErrorNode
}

func (s *errColl) VisitTerminal(node antlr.TerminalNode)      {}
func (s *errColl) EnterEveryRule(ctx antlr.ParserRuleContext) {}
func (s *errColl) ExitEveryRule(ctx antlr.ParserRuleContext)  {}
func (s *errColl) VisitErrorNode(node antlr.ErrorNode) {
	s.errs = append(s.errs, node)
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
