// Package passthrough hosts thin cwide subcommands that resolve any alias in
// the resource-type position and delegate to `kubectl` for the actual work.
// This lets users type `kubectl cwide annotate pd my-pod key=val` when `pd`
// is a cwide alias for `pods`, without duplicating kubectl's flag surface.
package passthrough

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/kubectl-cwide/pkg/utils"
)

// verbSpec describes one passthrough subcommand.
type verbSpec struct {
	Use     string
	Aliases []string
	Short   string
	Example string
}

var verbs = []verbSpec{
	{"annotate TYPE NAME KEY_1=VAL_1 [KEY_N=VAL_N] [flags]", nil, "Update annotations on a resource (aliases resolved, delegates to kubectl)",
		"  kubectl cwide annotate pd my-pod owner=alice"},
	{"label TYPE NAME KEY_1=VAL_1 [KEY_N=VAL_N] [flags]", nil, "Update labels on a resource (aliases resolved, delegates to kubectl)",
		"  kubectl cwide label pd my-pod env=prod"},
	{"edit TYPE NAME [flags]", nil, "Edit a resource on the server (aliases resolved, delegates to kubectl)",
		"  kubectl cwide edit pd my-pod"},
	{"delete TYPE NAME [flags]", []string{"del"}, "Delete a resource (aliases resolved, delegates to kubectl)",
		"  kubectl cwide delete pd my-pod"},
	{"describe TYPE NAME [flags]", nil, "Show details about a resource (aliases resolved, delegates to kubectl)",
		"  kubectl cwide describe pd my-pod"},
	{"apply -f FILENAME [flags]", nil, "Apply a configuration to a resource (delegates to kubectl; alias resolution has no effect for -f)",
		"  kubectl cwide apply -f manifest.yaml"},
	{"logs (POD | TYPE/NAME) [flags]", nil, "Print container logs (aliases resolved, delegates to kubectl)",
		"  kubectl cwide logs pd/my-pod"},
	{"exec (POD | TYPE/NAME) [flags] -- COMMAND", nil, "Execute a command in a container (aliases resolved, delegates to kubectl)",
		"  kubectl cwide exec pd/my-pod -- ls /"},
	{"port-forward TYPE/NAME LOCAL:REMOTE", []string{"pf"}, "Forward local ports (aliases resolved, delegates to kubectl)",
		"  kubectl cwide port-forward pd/my-pod 8080:80"},
	{"scale TYPE NAME --replicas=N", nil, "Scale a resource (aliases resolved, delegates to kubectl)",
		"  kubectl cwide scale deploy my-app --replicas=3"},
	{"rollout SUBCOMMAND", nil, "Manage rollouts (aliases resolved, delegates to kubectl)",
		"  kubectl cwide rollout status deploy/my-app"},
}

// NewCommands returns one cobra.Command per verb defined in `verbs`. Each
// resolves any alias in args[0] (or args[0]'s TYPE/NAME prefix) and execs
// `kubectl <verb> <resolved args...> <trailing flags...>`.
func NewCommands() []*cobra.Command {
	out := make([]*cobra.Command, 0, len(verbs))
	for _, v := range verbs {
		v := v
		verbName := cobraFirstToken(v.Use)
		cmd := &cobra.Command{
			Use:                verbName + " " + trimFirstToken(v.Use),
			Aliases:            v.Aliases,
			Short:              v.Short,
			Example:            v.Example,
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				resolved := resolveArgs(args)
				full := append([]string{verbName}, resolved...)
				kc := exec.Command("kubectl", full...)
				kc.Stdin = os.Stdin
				kc.Stdout = cmd.OutOrStdout()
				kc.Stderr = cmd.ErrOrStderr()
				if err := kc.Run(); err != nil {
					// Preserve kubectl's exit code semantics.
					if ee, ok := err.(*exec.ExitError); ok {
						os.Exit(ee.ExitCode())
					}
					return fmt.Errorf("kubectl %s: %w", verbName, err)
				}
				return nil
			},
		}
		out = append(out, cmd)
	}
	return out
}

// resolveArgs walks the arg list and rewrites any token that either (a)
// exactly matches a configured alias, or (b) has the form "<alias>/<name>"
// where <alias> matches. Flag values (anything after a `-...`) are left alone.
func resolveArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	out := make([]string, len(args))
	// Only rewrite the first non-flag token to keep behavior conservative —
	// TYPE always appears there in the verbs we host.
	seenType := false
	for i, a := range args {
		if !seenType && !isFlag(a) {
			out[i] = rewriteResourceToken(a)
			seenType = true
			continue
		}
		out[i] = a
	}
	return out
}

// rewriteResourceToken resolves an alias in the leading resource-type token.
// Handles the two forms kubectl accepts: "TYPE" and "TYPE/NAME".
func rewriteResourceToken(tok string) string {
	if slash := indexRune(tok, '/'); slash >= 0 {
		head := tok[:slash]
		return utils.ResolveAliasString(head) + tok[slash:]
	}
	return utils.ResolveAliasString(tok)
}

func isFlag(s string) bool { return len(s) > 0 && s[0] == '-' }

func indexRune(s string, r rune) int {
	for i, c := range s {
		if c == r {
			return i
		}
	}
	return -1
}

func cobraFirstToken(use string) string {
	for i, c := range use {
		if c == ' ' {
			return use[:i]
		}
	}
	return use
}

func trimFirstToken(use string) string {
	first := cobraFirstToken(use)
	if len(use) <= len(first) {
		return ""
	}
	return use[len(first)+1:]
}
