package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ShowAppHelp_NoAuthor(t *testing.T) {
	output := new(bytes.Buffer)
	cmd := &Command{Writer: output}
	_ = ShowAppHelp(cmd)

	if bytes.Contains(output.Bytes(), []byte("AUTHOR(S):")) {
		t.Errorf("expected\n%snot to include %s", output.String(), "AUTHOR(S):")
	}
}

func Test_ShowAppHelp_NoVersion(t *testing.T) {
	output := new(bytes.Buffer)
	cmd := &Command{Writer: output}

	cmd.Version = ""

	_ = ShowAppHelp(cmd)

	if bytes.Contains(output.Bytes(), []byte("VERSION:")) {
		t.Errorf("expected\n%snot to include %s", output.String(), "VERSION:")
	}
}

func Test_ShowAppHelp_HideVersion(t *testing.T) {
	output := new(bytes.Buffer)
	cmd := &Command{Writer: output}

	cmd.HideVersion = true

	_ = ShowAppHelp(cmd)

	if bytes.Contains(output.Bytes(), []byte("VERSION:")) {
		t.Errorf("expected\n%snot to include %s", output.String(), "VERSION:")
	}
}

func Test_ShowAppHelp_MultiLineDescription(t *testing.T) {
	output := new(bytes.Buffer)
	cmd := &Command{Writer: output}

	cmd.HideVersion = true
	cmd.Description = "multi\n  line"

	_ = ShowAppHelp(cmd)

	if !bytes.Contains(output.Bytes(), []byte("DESCRIPTION:\n   multi\n     line")) {
		t.Errorf("expected\n%s\nto include\n%s", output.String(), "DESCRIPTION:\n   multi\n     line")
	}
}

func Test_Help_Custom_Flags(t *testing.T) {
	oldFlag := HelpFlag
	defer func() {
		HelpFlag = oldFlag
	}()

	HelpFlag = &BoolFlag{
		Name:    "help",
		Aliases: []string{"x"},
		Usage:   "show help",
	}

	out := &bytes.Buffer{}

	cmd := &Command{
		Flags: []Flag{
			&BoolFlag{Name: "foo", Aliases: []string{"h"}},
		},
		Action: func(_ context.Context, cmd *Command) error {
			assert.True(t, cmd.Bool("h"), "custom help flag not set")
			return nil
		},
		Writer: out,
	}

	_ = cmd.Run(buildTestContext(t), []string{"test", "-h"})
	require.Len(t, out.String(), 0)
}

func Test_Help_Nil_Flags(t *testing.T) {
	oldFlag := HelpFlag
	defer func() {
		HelpFlag = oldFlag
	}()
	HelpFlag = nil

	cmd := &Command{
		Action: func(_ context.Context, cmd *Command) error {
			return nil
		},
	}
	out := new(bytes.Buffer)
	cmd.Writer = out
	_ = cmd.Run(buildTestContext(t), []string{"test"})
	require.Len(t, out.String(), 0)
}

func Test_Version_Custom_Flags(t *testing.T) {
	oldFlag := VersionFlag
	defer func() {
		VersionFlag = oldFlag
	}()

	VersionFlag = &BoolFlag{
		Name:    "version",
		Aliases: []string{"V"},
		Usage:   "show version",
	}

	out := &bytes.Buffer{}
	cmd := &Command{
		Flags: []Flag{
			&BoolFlag{Name: "foo", Aliases: []string{"v"}},
		},
		Action: func(_ context.Context, cmd *Command) error {
			assert.True(t, cmd.Bool("v"), "custom version flag not set")
			return nil
		},
		Writer: out,
	}

	_ = cmd.Run(buildTestContext(t), []string{"test", "-v"})
	require.Len(t, out.String(), 0)
}

func Test_helpCommand_Action_ErrorIfNoTopic(t *testing.T) {
	cmd := &Command{
		flagSet: flag.NewFlagSet("test", 0),
	}

	_ = cmd.flagSet.Parse([]string{"foo"})

	err := helpCommandAction(context.Background(), cmd)
	require.Error(t, err, "expected error from helpCommandAction()")

	exitErr, ok := err.(*exitError)
	require.True(t, ok, "expected *exitError from helpCommandAction()")

	require.Contains(t, exitErr.Error(), "No help topic for", "expected an unknown help topic error")
	require.Equal(t, 3, exitErr.exitCode, "expected exit value = 3")
}

func Test_helpCommand_InHelpOutput(t *testing.T) {
	cmd := &Command{}
	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"test", "--help"})

	s := output.String()

	require.NotContains(t, s, "\nCOMMANDS:\nGLOBAL OPTIONS:\n", "empty COMMANDS section detected")
	require.Contains(t, s, "help, h", "missing \"help, h\"")
}

func TestHelpCommand_FullName(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		contains string
		skip     bool
	}{
		{
			name:     "app help's FullName",
			args:     []string{"app", "help", "help"},
			contains: "app help -",
		},
		{
			name:     "app help's FullName via flag",
			args:     []string{"app", "-h", "help"},
			contains: "app help -",
		},
		{
			name:     "cmd help's FullName",
			args:     []string{"app", "cmd", "help", "help"},
			contains: "app cmd help -",
			skip:     true, // FIXME: App Command collapse
		},
		{
			name:     "cmd help's FullName via flag",
			args:     []string{"app", "cmd", "-h", "help"},
			contains: "app cmd help -",
			skip:     true, // FIXME: App Command collapse
		},
		{
			name:     "subcmd help's FullName",
			args:     []string{"app", "cmd", "subcmd", "help", "help"},
			contains: "app cmd subcmd help -",
		},
		{
			name:     "subcmd help's FullName via flag",
			args:     []string{"app", "cmd", "subcmd", "-h", "help"},
			contains: "app cmd subcmd help -",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := &bytes.Buffer{}

			if tc.skip {
				t.SkipNow()
			}

			cmd := &Command{
				Name: "app",
				Commands: []*Command{
					{
						Name: "cmd",
						Commands: []*Command{
							{
								Name: "subcmd",
							},
						},
					},
				},
				Writer:    out,
				ErrWriter: out,
			}

			r := require.New(t)
			r.NoError(cmd.Run(buildTestContext(t), tc.args))
			r.Contains(out.String(), tc.contains)
		})
	}
}

func Test_helpCommand_HideHelpCommand(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &Command{
		Name:   "app",
		Writer: buf,
	}

	err := cmd.Run(buildTestContext(t), []string{"app", "help", "help"})
	assert.NoError(t, err)
	got := buf.String()
	notWant := "COMMANDS:"
	assert.NotContains(t, got, notWant)
}

func Test_helpCommand_HideHelpFlag(t *testing.T) {
	app := buildMinimalTestCommand()

	assert.Error(t, app.Run(buildTestContext(t), []string{"app", "help", "-h"}), "Expected flag error - Got nil")
}

func Test_helpSubcommand_Action_ErrorIfNoTopic(t *testing.T) {
	cmd := &Command{
		flagSet: flag.NewFlagSet("test", 0),
	}
	_ = cmd.flagSet.Parse([]string{"foo"})

	err := helpCommandAction(context.Background(), cmd)
	require.Error(t, err, "expected error from helpCommandAction(), but got nil")

	exitErr, ok := err.(*exitError)
	require.True(t, ok, "expected *exitError from helpCommandAction(), but instead got: %v", err.Error())

	require.Contains(t, exitErr.Error(), "No help topic for", "expected an unknown help topic error")
	require.Equal(t, 3, exitErr.exitCode, "unexpected exit value")
}

func TestShowAppHelp_CommandAliases(t *testing.T) {
	out := &bytes.Buffer{}

	cmd := &Command{
		Commands: []*Command{
			{
				Name:    "frobbly",
				Aliases: []string{"fr", "frob"},
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
		},
		Writer: out,
	}

	_ = cmd.Run(buildTestContext(t), []string{"foo", "--help"})
	require.Contains(t, out.String(), "frobbly, fr, frob")
}

func TestShowCommandHelp_AppendHelp(t *testing.T) {
	testCases := []struct {
		name            string
		hideHelp        bool
		hideHelpCommand bool
		args            []string
		verify          func(*testing.T, string)
	}{
		{
			name:     "with HideHelp",
			hideHelp: true,
			args:     []string{"app", "help"},
			verify: func(t *testing.T, outString string) {
				r := require.New(t)
				r.NotContains(outString, "help, h  Shows a list of commands or help for one command")
				r.NotContains(outString, "--help, -h  show help")
			},
		},
		{
			name:            "with HideHelpCommand",
			hideHelpCommand: true,
			args:            []string{"app", "--help"},
			verify: func(t *testing.T, outString string) {
				r := require.New(t)
				r.NotContains(outString, "help, h  Shows a list of commands or help for one command")
				r.Contains(outString, "--help, -h  show help")
			},
		},
		{
			name: "with Subcommand",
			args: []string{"app", "cmd", "help"},
			verify: func(t *testing.T, outString string) {
				r := require.New(t)
				r.Contains(outString, "help, h  Shows a list of commands or help for one command")
				r.Contains(outString, "--help, -h  show help")
			},
		},
		{
			name: "without Subcommand",
			args: []string{"app", "help"},
			verify: func(t *testing.T, outString string) {
				r := require.New(t)
				r.Contains(outString, "help, h  Shows a list of commands or help for one command")
				r.Contains(outString, "--help, -h  show help")
			},
		},
	}
	for _, tc := range testCases {
		out := &bytes.Buffer{}

		t.Run(tc.name, func(t *testing.T) {
			cmd := &Command{
				Name:            "app",
				HideHelp:        tc.hideHelp,
				HideHelpCommand: tc.hideHelpCommand,
				Commands: []*Command{
					{
						Name:            "cmd",
						HideHelp:        tc.hideHelp,
						HideHelpCommand: tc.hideHelpCommand,
						Commands:        []*Command{{Name: "subcmd"}},
					},
				},
				Writer:    out,
				ErrWriter: out,
			}

			_ = cmd.Run(buildTestContext(t), tc.args)
			tc.verify(t, out.String())
		})
	}
}

func TestShowCommandHelp_HelpPrinter(t *testing.T) {
	/*doublecho := func(text string) string {
		return text + " " + text
	}*/

	tests := []struct {
		name         string
		template     string
		printer      helpPrinter
		command      string
		wantTemplate string
		wantOutput   string
	}{
		{
			name:     "no-command",
			template: "",
			printer: func(w io.Writer, _ string, _ interface{}) {
				fmt.Fprint(w, "yo")
			},
			command:      "",
			wantTemplate: RootCommandHelpTemplate,
			wantOutput:   "yo",
		},
		/*{
			name:     "standard-command",
			template: "",
			printer: func(w io.Writer, templ string, data interface{}) {
				fmt.Fprint(w, "yo")
			},
			command:      "my-command",
			wantTemplate: CommandHelpTemplate,
			wantOutput:   "yo",
		},
		{
			name:     "custom-template-command",
			template: "{{doublecho .Name}}",
			printer: func(w io.Writer, templ string, data interface{}) {
				// Pass a custom function to ensure it gets used
				fm := map[string]interface{}{"doublecho": doublecho}
				HelpPrinterCustom(w, templ, data, fm)
			},
			command:      "my-command",
			wantTemplate: "{{doublecho .Name}}",
			wantOutput:   "my-command my-command",
		},*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func(old helpPrinter) {
				HelpPrinter = old
			}(HelpPrinter)
			HelpPrinter = func(w io.Writer, templ string, data interface{}) {
				assert.Equal(t, tt.wantTemplate, templ, "template mismatch")
				tt.printer(w, templ, data)
			}

			var buf bytes.Buffer
			cmd := &Command{
				Name:   "my-app",
				Writer: &buf,
				Commands: []*Command{
					{
						Name:               "my-command",
						CustomHelpTemplate: tt.template,
					},
				},
			}

			err := cmd.Run(buildTestContext(t), []string{"my-app", "help", tt.command})
			require.NoError(t, err)

			got := buf.String()
			assert.Equal(t, tt.wantOutput, got)
		})
	}
}

func TestShowCommandHelp_HelpPrinterCustom(t *testing.T) {
	doublecho := func(text string) string {
		return text + " " + text
	}

	testCases := []struct {
		name         string
		template     string
		printer      helpPrinterCustom
		arguments    []string
		wantTemplate string
		wantOutput   string
	}{
		{
			name: "no command",
			printer: func(w io.Writer, _ string, _ any, _ map[string]any) {
				fmt.Fprint(w, "yo")
			},
			arguments:    []string{"my-app", "help"},
			wantTemplate: RootCommandHelpTemplate,
			wantOutput:   "yo",
		},
		{
			name: "standard command",
			printer: func(w io.Writer, _ string, _ any, _ map[string]any) {
				fmt.Fprint(w, "yo")
			},
			arguments:    []string{"my-app", "help", "my-command"},
			wantTemplate: SubcommandHelpTemplate,
			wantOutput:   "yo",
		},
		{
			name:     "custom template command",
			template: "{{doublecho .Name}}",
			printer: func(w io.Writer, templ string, data any, _ map[string]any) {
				// Pass a custom function to ensure it gets used
				fm := map[string]any{"doublecho": doublecho}
				printHelpCustom(w, templ, data, fm)
			},
			arguments:    []string{"my-app", "help", "my-command"},
			wantTemplate: "{{doublecho .Name}}",
			wantOutput:   "my-command my-command",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)

			defer func(old helpPrinterCustom) {
				HelpPrinterCustom = old
			}(HelpPrinterCustom)

			HelpPrinterCustom = func(w io.Writer, tmpl string, data any, fm map[string]any) {
				r.Nil(fm)
				r.Equal(tc.wantTemplate, tmpl)

				tc.printer(w, tmpl, data, fm)
			}

			out := &bytes.Buffer{}
			cmd := &Command{
				Name:   "my-app",
				Writer: out,
				Commands: []*Command{
					{
						Name:               "my-command",
						CustomHelpTemplate: tc.template,
					},
				},
			}

			t.Logf("cmd.Run(ctx, %+[1]v)", tc.arguments)

			r.NoError(cmd.Run(buildTestContext(t), tc.arguments))
			r.Equal(tc.wantOutput, out.String())
		})
	}
}

func TestShowCommandHelp_CommandAliases(t *testing.T) {
	out := &bytes.Buffer{}

	cmd := &Command{
		Commands: []*Command{
			{
				Name:    "frobbly",
				Aliases: []string{"fr", "frob", "bork"},
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
		},
		Writer: out,
	}

	_ = cmd.Run(buildTestContext(t), []string{"foo", "help", "fr"})
	require.Contains(t, out.String(), "frobbly")
}

func TestShowSubcommandHelp_CommandAliases(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name:    "frobbly",
				Aliases: []string{"fr", "frob", "bork"},
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo", "help"})

	assert.Contains(t, output.String(), "frobbly, fr, frob, bork", "expected output to include all command aliases")
}

func TestShowCommandHelp_Customtemplate(t *testing.T) {
	cmd := &Command{
		Name: "foo",
		Commands: []*Command{
			{
				Name: "frobbly",
				Action: func(context.Context, *Command) error {
					return nil
				},
				CustomHelpTemplate: `NAME:
   {{.FullName}} - {{.Usage}}

USAGE:
   {{.FullName}} [FLAGS] TARGET [TARGET ...]

FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}
EXAMPLES:
   1. Frobbly runs with this param locally.
      $ {{.FullName}} wobbly
`,
			},
		},
	}
	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo", "help", "frobbly"})

	assert.NotContains(t, output.String(), "2. Frobbly runs without this param locally.",
		"expected output to exclude \"2. Frobbly runs without this param locally.\";")

	assert.Contains(t, output.String(), "1. Frobbly runs with this param locally.",
		"expected output to include \"1. Frobbly runs with this param locally.\"")

	assert.Contains(t, output.String(), "$ foo frobbly wobbly",
		"expected output to include \"$ foo frobbly wobbly\"")
}

func TestShowSubcommandHelp_CommandUsageText(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name:      "frobbly",
				UsageText: "this is usage text",
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo", "frobbly", "--help"})

	assert.Contains(t, output.String(), "this is usage text",
		"expected output to include usage text")
}

func TestShowSubcommandHelp_MultiLine_CommandUsageText(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name: "frobbly",
				UsageText: `This is a
multi
line
UsageText`,
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo", "frobbly", "--help"})

	expected := `USAGE:
   This is a
   multi
   line
   UsageText
`

	assert.Contains(t, output.String(), expected,
		"expected output to include usage text")
}

func TestShowSubcommandHelp_SubcommandUsageText(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name: "frobbly",
				Commands: []*Command{
					{
						Name:      "bobbly",
						UsageText: "this is usage text",
					},
				},
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo", "frobbly", "bobbly", "--help"})

	assert.Contains(t, output.String(), "this is usage text",
		"expected output to include usage text")
}

func TestShowSubcommandHelp_MultiLine_SubcommandUsageText(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name: "frobbly",
				Commands: []*Command{
					{
						Name: "bobbly",
						UsageText: `This is a
multi
line
UsageText`,
					},
				},
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo", "frobbly", "bobbly", "--help"})

	expected := `USAGE:
   This is a
   multi
   line
   UsageText
`

	assert.Contains(t, output.String(), expected,
		"expected output to include usage text")
}

func TestShowAppHelp_HiddenCommand(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name: "frobbly",
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
			{
				Name:   "secretfrob",
				Hidden: true,
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"app", "--help"})

	assert.NotContains(t, output.String(), "secretfrob",
		"expected output to exclude \"secretfrob\"")

	assert.Contains(t, output.String(), "frobbly",
		"expected output to include \"frobbly\"")
}

func TestShowAppHelp_HelpPrinter(t *testing.T) {
	doublecho := func(text string) string {
		return text + " " + text
	}

	tests := []struct {
		name         string
		template     string
		printer      helpPrinter
		wantTemplate string
		wantOutput   string
	}{
		{
			name:     "standard-command",
			template: "",
			printer: func(w io.Writer, _ string, _ interface{}) {
				fmt.Fprint(w, "yo")
			},
			wantTemplate: RootCommandHelpTemplate,
			wantOutput:   "yo",
		},
		{
			name:     "custom-template-command",
			template: "{{doublecho .Name}}",
			printer: func(w io.Writer, templ string, data interface{}) {
				// Pass a custom function to ensure it gets used
				fm := map[string]interface{}{"doublecho": doublecho}
				printHelpCustom(w, templ, data, fm)
			},
			wantTemplate: "{{doublecho .Name}}",
			wantOutput:   "my-app my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func(old helpPrinter) {
				HelpPrinter = old
			}(HelpPrinter)
			HelpPrinter = func(w io.Writer, templ string, data interface{}) {
				assert.Equal(t, tt.wantTemplate, templ, "unexpected template")
				tt.printer(w, templ, data)
			}

			var buf bytes.Buffer
			cmd := &Command{
				Name:                          "my-app",
				Writer:                        &buf,
				CustomRootCommandHelpTemplate: tt.template,
			}

			err := cmd.Run(buildTestContext(t), []string{"my-app", "help"})
			require.NoError(t, err)

			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestShowAppHelp_HelpPrinterCustom(t *testing.T) {
	doublecho := func(text string) string {
		return text + " " + text
	}

	tests := []struct {
		name         string
		template     string
		printer      helpPrinterCustom
		wantTemplate string
		wantOutput   string
	}{
		{
			name:     "standard-command",
			template: "",
			printer: func(w io.Writer, _ string, _ interface{}, _ map[string]interface{}) {
				fmt.Fprint(w, "yo")
			},
			wantTemplate: RootCommandHelpTemplate,
			wantOutput:   "yo",
		},
		{
			name:     "custom-template-command",
			template: "{{doublecho .Name}}",
			printer: func(w io.Writer, templ string, data interface{}, _ map[string]interface{}) {
				// Pass a custom function to ensure it gets used
				fm := map[string]interface{}{"doublecho": doublecho}
				printHelpCustom(w, templ, data, fm)
			},
			wantTemplate: "{{doublecho .Name}}",
			wantOutput:   "my-app my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func(old helpPrinterCustom) {
				HelpPrinterCustom = old
			}(HelpPrinterCustom)
			HelpPrinterCustom = func(w io.Writer, templ string, data interface{}, fm map[string]interface{}) {
				assert.Nil(t, fm, "unexpected function map passed")
				assert.Equal(t, tt.wantTemplate, templ, "unexpected template")
				tt.printer(w, templ, data, fm)
			}

			var buf bytes.Buffer
			cmd := &Command{
				Name:                          "my-app",
				Writer:                        &buf,
				CustomRootCommandHelpTemplate: tt.template,
			}

			err := cmd.Run(buildTestContext(t), []string{"my-app", "help"})
			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestShowAppHelp_CustomAppTemplate(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name: "frobbly",
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
			{
				Name:   "secretfrob",
				Hidden: true,
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
		},
		ExtraInfo: func() map[string]string {
			platform := fmt.Sprintf("OS: %s | Arch: %s", runtime.GOOS, runtime.GOARCH)
			goruntime := fmt.Sprintf("Version: %s | CPUs: %d", runtime.Version(), runtime.NumCPU())
			return map[string]string{
				"PLATFORM": platform,
				"RUNTIME":  goruntime,
			}
		},
		CustomRootCommandHelpTemplate: `NAME:
  {{.Name}} - {{.Usage}}

USAGE:
  {{.Name}} {{if .VisibleFlags}}[FLAGS] {{end}}COMMAND{{if .VisibleFlags}} [COMMAND FLAGS | -h]{{end}} [ARGUMENTS...]

COMMANDS:
  {{range .VisibleCommands}}{{join .Names ", "}}{{ "\t" }}{{.Usage}}
  {{end}}{{if .VisibleFlags}}
GLOBAL FLAGS:
  {{range .VisibleFlags}}{{.}}
  {{end}}{{end}}
VERSION:
  2.0.0
{{"\n"}}{{range $key, $value := ExtraInfo}}
{{$key}}:
  {{$value}}
{{end}}`,
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"app", "--help"})

	assert.NotContains(t, output.String(), "secretfrob", "expected output to exclude \"secretfrob\"")
	assert.Contains(t, output.String(), "frobbly", "expected output to include \"frobbly\"")

	if !strings.Contains(output.String(), "PLATFORM:") ||
		!strings.Contains(output.String(), "OS:") ||
		!strings.Contains(output.String(), "Arch:") {
		t.Errorf("expected output to include \"PLATFORM:, OS: and Arch:\"; got: %q", output.String())
	}

	if !strings.Contains(output.String(), "RUNTIME:") ||
		!strings.Contains(output.String(), "Version:") ||
		!strings.Contains(output.String(), "CPUs:") {
		t.Errorf("expected output to include \"RUNTIME:, Version: and CPUs:\"; got: %q", output.String())
	}

	if !strings.Contains(output.String(), "VERSION:") ||
		!strings.Contains(output.String(), "2.0.0") {
		t.Errorf("expected output to include \"VERSION:, 2.0.0\"; got: %q", output.String())
	}
}

func TestShowAppHelp_UsageText(t *testing.T) {
	cmd := &Command{
		UsageText: "This is a single line of UsageText",
		Commands: []*Command{
			{
				Name: "frobbly",
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo"})

	assert.Contains(t, output.String(), "This is a single line of UsageText", "expected output to include usage text")
}

func TestShowAppHelp_MultiLine_UsageText(t *testing.T) {
	cmd := &Command{
		UsageText: `This is a
multi
line
App UsageText`,
		Commands: []*Command{
			{
				Name: "frobbly",
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo"})

	expected := `USAGE:
   This is a
   multi
   line
   App UsageText
`

	assert.Contains(t, output.String(), expected, "expected output to include usage text")
}

func TestShowAppHelp_CommandMultiLine_UsageText(t *testing.T) {
	cmd := &Command{
		UsageText: `This is a
multi
line
App UsageText`,
		Commands: []*Command{
			{
				Name:    "frobbly",
				Aliases: []string{"frb1", "frbb2", "frl2"},
				Usage:   "this is a long help output for the run command, long usage \noutput, long usage output, long usage output, long usage output\noutput, long usage output, long usage output",
			},
			{
				Name:    "grobbly",
				Aliases: []string{"grb1", "grbb2"},
				Usage:   "this is another long help output for the run command, long usage \noutput, long usage output",
			},
		},
	}

	output := &bytes.Buffer{}
	cmd.Writer = output

	_ = cmd.Run(buildTestContext(t), []string{"foo"})

	expected := "COMMANDS:\n" +
		"   frobbly, frb1, frbb2, frl2  this is a long help output for the run command, long usage \n" +
		"                               output, long usage output, long usage output, long usage output\n" +
		"                               output, long usage output, long usage output\n" +
		"   grobbly, grb1, grbb2        this is another long help output for the run command, long usage \n" +
		"                               output, long usage output"
	assert.Contains(t, output.String(), expected, "expected output to include usage text")
}

func TestHideHelpCommand(t *testing.T) {
	cmd := &Command{
		HideHelpCommand: true,
		Writer:          io.Discard,
	}

	err := cmd.Run(buildTestContext(t), []string{"foo", "help"})
	require.ErrorContains(t, err, "No help topic for 'help'")

	err = cmd.Run(buildTestContext(t), []string{"foo", "--help"})
	assert.NoError(t, err)
}

func TestHideHelpCommand_False(t *testing.T) {
	cmd := &Command{
		HideHelpCommand: false,
		Writer:          io.Discard,
	}

	err := cmd.Run(buildTestContext(t), []string{"foo", "help"})
	assert.NoError(t, err)

	err = cmd.Run(buildTestContext(t), []string{"foo", "--help"})
	assert.NoError(t, err)
}

func TestHideHelpCommand_WithHideHelp(t *testing.T) {
	cmd := &Command{
		HideHelp:        true, // effective (hides both command and flag)
		HideHelpCommand: true, // ignored
		Writer:          io.Discard,
	}

	err := cmd.Run(buildTestContext(t), []string{"foo", "help"})
	require.ErrorContains(t, err, "No help topic for 'help'")

	err = cmd.Run(buildTestContext(t), []string{"foo", "--help"})
	require.ErrorContains(t, err, "flag: help requested")
}

func TestHideHelpCommand_WithSubcommands(t *testing.T) {
	cmd := &Command{
		HideHelpCommand: true,
		Commands: []*Command{
			{
				Name: "nully",
				Commands: []*Command{
					{
						Name: "nully2",
					},
				},
			},
		},
	}

	r := require.New(t)

	r.ErrorContains(cmd.Run(buildTestContext(t), []string{"cli.test", "help"}), "No help topic for 'help'")
	r.NoError(cmd.Run(buildTestContext(t), []string{"cli.test", "--help"}))
}

func TestDefaultCompleteWithFlags(t *testing.T) {
	origArgv := os.Args
	t.Cleanup(func() { os.Args = origArgv })

	t.Setenv("SHELL", "bash")

	for _, tc := range []struct {
		name     string
		cmd      *Command
		argv     []string
		expected string
	}{
		{
			name:     "empty",
			cmd:      &Command{},
			argv:     []string{"prog", "cmd"},
			expected: "",
		},
		{
			name: "typical-flag-suggestion",
			cmd: &Command{
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&IntFlag{Name: "everybody-jump-on"},
					},
					Commands: []*Command{
						{Name: "putz"},
					},
				},
			},
			argv:     []string{"cmd", "--e", "--generate-shell-completion"},
			expected: "--excitement\n",
		},
		{
			name: "typical-command-suggestion",
			cmd: &Command{
				Name: "putz",
				Commands: []*Command{
					{Name: "futz"},
				},
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&IntFlag{Name: "everybody-jump-on"},
					},
				},
			},
			argv:     []string{"cmd", "--generate-shell-completion"},
			expected: "futz\n",
		},
		{
			name: "autocomplete-with-spaces",
			cmd: &Command{
				Name: "putz",
				Commands: []*Command{
					{Name: "help"},
				},
				Flags: []Flag{
					&BoolFlag{Name: "excitement"},
					&StringFlag{Name: "hat-shape"},
				},
				parent: &Command{
					Name: "cmd",
					Flags: []Flag{
						&BoolFlag{Name: "happiness"},
						&IntFlag{Name: "everybody-jump-on"},
					},
				},
			},
			argv:     []string{"cmd", "--url", "http://localhost:8000", "h", "--generate-shell-completion"},
			expected: "help\n",
		},
	} {
		t.Run(tc.name, func(ct *testing.T) {
			writer := &bytes.Buffer{}
			rootCmd := tc.cmd.Root()
			rootCmd.Writer = writer

			os.Args = tc.argv
			f := DefaultCompleteWithFlags(tc.cmd)
			f(context.Background(), tc.cmd)

			written := writer.String()

			assert.Equal(t, tc.expected, written, "written help does not match")
		})
	}
}

func TestWrap(t *testing.T) {
	emptywrap := wrap("", 4, 16)
	assert.Empty(t, emptywrap, "Wrapping empty line should return empty line")
}

func TestWrappedHelp(t *testing.T) {
	// Reset HelpPrinter after this test.
	defer func(old helpPrinter) {
		HelpPrinter = old
	}(HelpPrinter)

	output := new(bytes.Buffer)
	cmd := &Command{
		Writer: output,
		Flags: []Flag{
			&BoolFlag{
				Name:    "foo",
				Aliases: []string{"h"},
				Usage:   "here's a really long help text line, let's see where it wraps. blah blah blah and so on.",
			},
		},
		Usage:     "here's a sample App.Usage string long enough that it should be wrapped in this test",
		UsageText: "i'm not sure how App.UsageText differs from App.Usage, but this should also be wrapped in this test",
		// TODO: figure out how to make ArgsUsage appear in the help text, and test that
		Description: `here's a sample App.Description string long enough that it should be wrapped in this test

with a newline
   and an indented line`,
		Copyright: `Here's a sample copyright text string long enough that it should be wrapped.
Including newlines.
   And also indented lines.


And then another long line. Blah blah blah does anybody ever read these things?`,
	}

	HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		funcMap := map[string]interface{}{
			"wrapAt": func() int {
				return 30
			},
		}

		HelpPrinterCustom(w, templ, data, funcMap)
	}

	_ = ShowAppHelp(cmd)

	expected := `NAME:
    - here's a sample
      App.Usage string long
      enough that it should be
      wrapped in this test

USAGE:
   i'm not sure how
   App.UsageText differs from
   App.Usage, but this should
   also be wrapped in this
   test

DESCRIPTION:
   here's a sample
   App.Description string long
   enough that it should be
   wrapped in this test

   with a newline
      and an indented line

GLOBAL OPTIONS:
   --foo, -h here's a
      really long help text
      line, let's see where it
      wraps. blah blah blah
      and so on. (default:
      false)

COPYRIGHT:
   Here's a sample copyright
   text string long enough
   that it should be wrapped.
   Including newlines.
      And also indented lines.


   And then another long line.
   Blah blah blah does anybody
   ever read these things?
`

	assert.Equal(t, expected, output.String(), "Unexpected wrapping")
}

func TestWrappedCommandHelp(t *testing.T) {
	// Reset HelpPrinter after this test.
	defer func(old helpPrinter) {
		HelpPrinter = old
	}(HelpPrinter)

	output := &bytes.Buffer{}
	cmd := &Command{
		Writer:    output,
		ErrWriter: output,
		Commands: []*Command{
			{
				Name:        "add",
				Aliases:     []string{"a"},
				Usage:       "add a task to the list",
				UsageText:   "this is an even longer way of describing adding a task to the list",
				Description: "and a description long enough to wrap in this test case",
				Action: func(context.Context, *Command) error {
					return nil
				},
			},
		},
	}
	cmd.setupDefaults([]string{"cli.test"})
	cmd.setupCommandGraph()

	HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		funcMap := map[string]interface{}{
			"wrapAt": func() int {
				return 30
			},
		}

		HelpPrinterCustom(w, templ, data, funcMap)
	}

	r := require.New(t)

	r.NoError(ShowCommandHelp(context.Background(), cmd, "add"))
	r.Equal(`NAME:
   cli.test add - add a task
                  to the list

USAGE:
   this is an even longer way
   of describing adding a task
   to the list

DESCRIPTION:
   and a description long
   enough to wrap in this test
   case

COMMANDS:
   help, h  Shows a list of
            commands or help
            for one command

OPTIONS:
   --help, -h show help
      (default: false)
`,
		output.String(),
	)
}

func TestWrappedSubcommandHelp(t *testing.T) {
	// Reset HelpPrinter after this test.
	defer func(old helpPrinter) {
		HelpPrinter = old
	}(HelpPrinter)

	output := new(bytes.Buffer)
	cmd := &Command{
		Name:   "cli.test",
		Writer: output,
		Commands: []*Command{
			{
				Name:        "bar",
				Aliases:     []string{"a"},
				Usage:       "add a task to the list",
				UsageText:   "this is an even longer way of describing adding a task to the list",
				Description: "and a description long enough to wrap in this test case",
				Action: func(context.Context, *Command) error {
					return nil
				},
				Commands: []*Command{
					{
						Name:      "grok",
						Usage:     "remove an existing template",
						UsageText: "longer usage text goes here, la la la, hopefully this is long enough to wrap even more",
						Action: func(context.Context, *Command) error {
							return nil
						},
					},
				},
			},
		},
	}

	HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		funcMap := map[string]interface{}{
			"wrapAt": func() int {
				return 30
			},
		}

		HelpPrinterCustom(w, templ, data, funcMap)
	}

	_ = cmd.Run(buildTestContext(t), []string{"foo", "bar", "grok", "--help"})

	expected := `NAME:
   cli.test bar grok - remove
                       an
                       existing
                       template

USAGE:
   longer usage text goes
   here, la la la, hopefully
   this is long enough to wrap
   even more

OPTIONS:
   --help, -h show help
      (default: false)
`

	assert.Equal(t, expected, output.String(), "Unexpected wrapping")
}

func TestWrappedHelpSubcommand(t *testing.T) {
	// Reset HelpPrinter after this test.
	defer func(old helpPrinter) {
		HelpPrinter = old
	}(HelpPrinter)

	output := &bytes.Buffer{}
	cmd := &Command{
		Name:      "cli.test",
		Writer:    output,
		ErrWriter: output,
		Commands: []*Command{
			{
				Name:        "bar",
				Aliases:     []string{"a"},
				Usage:       "add a task to the list",
				UsageText:   "this is an even longer way of describing adding a task to the list",
				Description: "and a description long enough to wrap in this test case",
				Action: func(context.Context, *Command) error {
					return nil
				},
				Commands: []*Command{
					{
						Name:      "grok",
						Usage:     "remove an existing template",
						UsageText: "longer usage text goes here, la la la, hopefully this is long enough to wrap even more",
						Action: func(context.Context, *Command) error {
							return nil
						},
						Flags: []Flag{
							&StringFlag{
								Name:  "test-f",
								Usage: "my test usage",
							},
						},
					},
				},
			},
		},
	}

	HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		funcMap := map[string]interface{}{
			"wrapAt": func() int {
				return 30
			},
		}

		HelpPrinterCustom(w, templ, data, funcMap)
	}

	r := require.New(t)

	r.NoError(cmd.Run(buildTestContext(t), []string{"cli.test", "bar", "help", "grok"}))
	r.Equal(`NAME:
   cli.test bar grok - remove
                       an
                       existing
                       template

USAGE:
   longer usage text goes
   here, la la la, hopefully
   this is long enough to wrap
   even more

COMMANDS:
   help, h  Shows a list of
            commands or help
            for one command

OPTIONS:
   --test-f value my test
      usage
   --help, -h show help
      (default: false)
`,
		output.String(),
	)
}

func TestCategorizedHelp(t *testing.T) {
	// Reset HelpPrinter after this test.
	defer func(old helpPrinter) {
		HelpPrinter = old
	}(HelpPrinter)

	output := new(bytes.Buffer)
	cmd := &Command{
		Name:   "cli.test",
		Writer: output,
		Action: func(context.Context, *Command) error { return nil },
		Flags: []Flag{
			&StringFlag{
				Name: "strd", // no category set
			},
			&IntFlag{
				Name:     "intd",
				Aliases:  []string{"altd1", "altd2"},
				Category: "cat1",
			},
		},
	}

	HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		funcMap := map[string]interface{}{
			"wrapAt": func() int {
				return 30
			},
		}

		HelpPrinterCustom(w, templ, data, funcMap)
	}

	r := require.New(t)
	r.NoError(cmd.Run(buildTestContext(t), []string{"cli.test", "help"}))

	r.Equal(`NAME:
   cli.test - A new cli
              application

USAGE:
   cli.test [global options] [command [command options]] [arguments...]

COMMANDS:
   help, h  Shows a list of
            commands or help
            for one command

GLOBAL OPTIONS:
   --help, -h    show help (default: false)
   --strd value  

   cat1

   --intd value, --altd1 value, --altd2 value  (default: 0)

`, output.String())
}
