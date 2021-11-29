package iac

import (
	"encoding/json"
	"fmt"
	"github.com/snyk/snyk-lsp/lsp"
	"github.com/snyk/snyk-lsp/util"
	sglsp "github.com/sourcegraph/go-lsp"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	severities = map[string]sglsp.DiagnosticSeverity{
		"high": sglsp.Error,
		"low":  sglsp.Warning,
	}
)

func HandleFile(uri sglsp.DocumentURI) ([]lsp.Diagnostic, []sglsp.CodeLens, error) {
	diagnostics, codeLenses, err := fetch(strings.ReplaceAll(string(uri), "file://", ""))
	return diagnostics, codeLenses, err
}

func fetch(path string) ([]lsp.Diagnostic, []sglsp.CodeLens, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	resBytes, err := exec.Command(util.CliPath, "iac", "test", path, "--json").CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() > 1 {
				return nil, nil, fmt.Errorf("error running fetch: %s: %s", err, string(resBytes))
			}
		} else {
			return nil, nil, fmt.Errorf("error running fetch: %s: %s", err, string(resBytes))
		}
	}
	var res testResult
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return nil, nil, err
	}
	diagnostics := convertDiagnostics(res)
	codeLenses := convertCodeLenses(res)
	return diagnostics, codeLenses, nil
}

func convertCodeLenses(res testResult) []sglsp.CodeLens {
	var lenses []sglsp.CodeLens
	for _, issue := range res.IacIssues {
		lens := sglsp.CodeLens{
			Range: sglsp.Range{
				Start: sglsp.Position{Line: issue.LineNumber - 1, Character: 0},
				End:   sglsp.Position{Line: issue.LineNumber - 1, Character: 80},
			},
			Command: sglsp.Command{
				Title:   "Show Description of " + issue.PublicID,
				Command: "snyk.launchBrowser",
				Arguments: []interface{}{
					issue.Documentation,
				},
			},
		}
		lenses = append(lenses, lens)
	}
	return lenses
}

func convertDiagnostics(res testResult) []lsp.Diagnostic {
	var diagnostics []lsp.Diagnostic
	for _, issue := range res.IacIssues {
		diagnostic := lsp.Diagnostic{
			Source:  "Snyk LSP",
			Message: fmt.Sprintf("%s: %s", issue.PublicID, issue.Title),
			Range: sglsp.Range{
				Start: sglsp.Position{Line: issue.LineNumber - 1, Character: 0},
				End:   sglsp.Position{Line: issue.LineNumber - 1, Character: 80},
			},
			Severity: lspSeverity(issue.Severity),
			// don't use for now as it's not widely supported
			//CodeDescription: lsp.CodeDescription{
			//	Href: issue.Documentation,
			//},
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}

type testResult struct {
	IacIssues []struct {
		PublicID      string  `json:"publicId"`
		Title         string  `json:"title"`
		Severity      string  `json:"severity"`
		LineNumber    int     `json:"lineNumber"`
		Documentation lsp.Uri `json:"documentation"`
	} `json:"infrastructureAsCodeIssues"`
}

func lspSeverity(snykSeverity string) sglsp.DiagnosticSeverity {
	lspSev, ok := severities[snykSeverity]
	if !ok {
		return sglsp.Info
	}
	return lspSev
}
