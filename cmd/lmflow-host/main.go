package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	lmcode "github.com/lmctl-ai/lmcode-go"
	"github.com/lmctl-ai/n8n/internal/host"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, stdout io.Writer) int {
	if len(args) == 2 && args[0] == "--lmcode-schema" && args[1] == "--json" {
		if _, err := stdout.Write(host.Schema); err != nil {
			return 4
		}
		return 0
	}
	if len(args) == 1 && args[0] == "--lmcode-signatures" {
		registry, err := host.NewRegistry(host.Config{})
		if err != nil {
			return lmcode.FailureFrame(stdout, "cli", err)
		}
		for _, signature := range registry.Signatures() {
			if _, err := fmt.Fprintln(stdout, signature); err != nil {
				return 4
			}
		}
		return 0
	}
	return execute(args, stdout)
}

func execute(args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("lmflow-host", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	codeFile := flags.String("lmcode", "", "LMCode program")
	workflowFile := flags.String("workflow", "", "workflow JSON")
	connectorFile := flags.String("connector", "", "connector JSON")
	credentialsFile := flags.String("credentials", "", "credentials JSON")
	baseURL := flags.String("base-url", "", "trusted base URL override")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *codeFile == "" || *workflowFile == "" || *connectorFile == "" || *credentialsFile == "" {
		return lmcode.FailureFrame(stdout, "cli", lmcode.NewError(lmcode.CLI_USAGE, "invalid execution flags"))
	}
	codePath, err := filepath.Abs(*codeFile)
	if err != nil {
		return lmcode.FailureFrame(stdout, "cli", lmcode.NewError(lmcode.CLI_USAGE, "invalid LMCode path"))
	}
	results, err := lmcode.InvocationResultsFromCommand([]string{codePath})
	if err != nil {
		return lmcode.FailureFrame(stdout, "cli", err)
	}
	index := 0
	phase := "load"
	fail := func(err error) int {
		_ = results.Fail(&index, phase, err)
		return results.WriteFrame(stdout)
	}
	code, err := os.ReadFile(codePath)
	if err != nil {
		return fail(lmcode.NewError(lmcode.LOAD_BAD_BYTECODE, "cannot read LMCode program"))
	}
	config, err := host.LoadConfig(*workflowFile, *connectorFile, *credentialsFile, *baseURL)
	if err != nil {
		return fail(err)
	}
	registry, err := host.NewRegistry(config)
	if err != nil {
		return fail(err)
	}
	program, err := lmcode.Load(code, host.Schema, registry)
	if err != nil {
		return fail(err)
	}
	contract, err := lmcode.ResultContractFromSchema(host.Schema)
	if err != nil {
		return fail(err)
	}
	if _, err = results.Select(0, registry, contract); err != nil {
		return fail(err)
	}
	phase = "run"
	context := lmcode.NewContext()
	context.SetDry(results.Dry())
	if err = program.Run(context, registry); err != nil {
		return fail(err)
	}
	if err = results.Complete(0); err != nil {
		return fail(err)
	}
	return results.WriteFrame(stdout)
}
