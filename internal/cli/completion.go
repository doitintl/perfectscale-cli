package cli

import (
	"fmt"

	"github.com/perfectscale/poc-cli/internal/clierr"
	"github.com/perfectscale/poc-cli/internal/config"
	ucli "github.com/urfave/cli/v2"
)

// bashCompletionScript and zshCompletionScript are adapted from
// urfave/cli/v2's autocomplete/{bash,zsh}_autocomplete templates, with the
// $PROG placeholder replaced by the fixed binary name at generation time —
// the app never runs under any other name, so there's no need to keep it as
// a runtime placeholder. Both shell out to `pscli --generate-bash-completion`
// at completion time, which App.EnableBashCompletion answers dynamically.
const bashCompletionScript = `#! /bin/bash

_cli_init_completion() {
  COMPREPLY=()
  _get_comp_words_by_ref "$@" cur prev words cword
}

_cli_bash_autocomplete() {
  if [[ "${COMP_WORDS[0]}" != "source" ]]; then
    local cur opts base words
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    if declare -F _init_completion >/dev/null 2>&1; then
      _init_completion -n "=:" || return
    else
      _cli_init_completion -n "=:" || return
    fi
    words=("${words[@]:0:$cword}")
    if [[ "$cur" == "-"* ]]; then
      requestComp="${words[*]} ${cur} --generate-bash-completion"
    else
      requestComp="${words[*]} --generate-bash-completion"
    fi
    opts=$(eval "${requestComp}" 2>/dev/null)
    COMPREPLY=($(compgen -W "${opts}" -- ${cur}))
    return 0
  fi
}

complete -o bashdefault -o default -o nospace -F _cli_bash_autocomplete %[1]s
`

const zshCompletionScript = `#compdef %[1]s

_cli_zsh_autocomplete() {
  local -a opts
  local cur
  cur=${words[-1]}
  if [[ "$cur" == "-"* ]]; then
    opts=("${(@f)$(${words[@]:0:#words[@]-1} ${cur} --generate-bash-completion)}")
  else
    opts=("${(@f)$(${words[@]:0:#words[@]-1} --generate-bash-completion)}")
  fi

  if [[ "${opts[1]}" != "" ]]; then
    _describe 'values' opts
  else
    _files
  fi
}

compdef _cli_zsh_autocomplete %[1]s
`

const powershellCompletionScript = `Register-ArgumentCompleter -Native -CommandName %[1]s -ScriptBlock {
     param($commandName, $wordToComplete, $cursorPosition)
     $other = "$wordToComplete --generate-bash-completion"
         Invoke-Expression $other | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
         }
 }
`

func completionCommand() *ucli.Command {
	return &ucli.Command{
		Name:      "completion",
		Usage:     "Print a shell completion script",
		ArgsUsage: "<bash|zsh|fish|powershell>",
		Description: withCommandName(`Prints a completion script for the given shell to stdout. bash, zsh, and
powershell scripts complete dynamically by shelling back into {{cmd}}; the
fish script is generated from the full command tree.

Examples:
  {{cmd}} completion bash
  {{cmd}} completion zsh
  {{cmd}} completion fish
  {{cmd}} completion powershell

Install:
  bash        echo 'eval "$({{cmd}} completion bash)"' >> ~/.bashrc
  zsh         echo 'eval "$({{cmd}} completion zsh)"' >> ~/.zshrc
  fish        {{cmd}} completion fish > ~/.config/fish/completions/{{cmd}}.fish
  powershell  Add 'Invoke-Expression (& {{cmd}} completion powershell | Out-String)' to $PROFILE

Output:
  Raw shell script on stdout; -o/-p/-d/-u are ignored.`),
		Action: runCompletion,
	}
}

func runCompletion(c *ucli.Context) error {
	args := c.Args().Slice()
	if len(args) != 1 {
		return clierr.Usage("completion requires exactly one shell argument: bash, zsh, fish, or powershell")
	}

	switch shell := args[0]; shell {
	case "bash":
		fmt.Fprintf(c.App.Writer, bashCompletionScript, config.BinaryName)
	case "zsh":
		fmt.Fprintf(c.App.Writer, zshCompletionScript, config.BinaryName)
	case "powershell":
		fmt.Fprintf(c.App.Writer, powershellCompletionScript, config.BinaryName)
	case "fish":
		script, err := c.App.ToFishCompletion()
		if err != nil {
			return fmt.Errorf("generate fish completion: %w", err)
		}

		fmt.Fprint(c.App.Writer, script)
	default:
		return clierr.Usage("unknown shell %q; supported: bash, zsh, fish, powershell", shell)
	}

	return nil
}
