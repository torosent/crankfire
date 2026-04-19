package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/pflag"

	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tagfilter"
)

// RunSession is the entry point for `crankfire session ...`.
func RunSession(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: crankfire session <list|edit> [args]")
		return ExitUsage
	}
	switch args[0] {
	case "list":
		return sessionList(ctx, st, args[1:], stdout, stderr)
	case "edit":
		return sessionEdit(ctx, st, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n", args[0])
		return ExitUsage
	}
}

func sessionList(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("session list", pflag.ContinueOnError)
	fs.SetOutput(stderr)
	var tagFlags []string
	fs.StringArrayVar(&tagFlags, "tag", nil, "filter expression (repeat for AND, comma for OR)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	matchers, err := buildMatchers(tagFlags)
	if err != nil {
		fmt.Fprintf(stderr, "tag filter: %v\n", err)
		return ExitUsage
	}
	sessions, err := st.ListSessions(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "list sessions: %v\n", err)
		return ExitRunnerError
	}
	fmt.Fprintf(stdout, "%-26s  %-30s  %s\n", "ID", "NAME", "TAGS")
	for _, sess := range sessions {
		if !matchAll(matchers, sess.Tags) {
			continue
		}
		fmt.Fprintf(stdout, "%-26s  %-30s  %s\n", sess.ID, sess.Name, strings.Join(sess.Tags, ","))
	}
	return ExitOK
}

func sessionEdit(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: crankfire session edit <id> [--add-tag T] [--remove-tag T] [--name N] [--description D]")
		return ExitUsage
	}
	id := args[0]
	fs := pflag.NewFlagSet("session edit", pflag.ContinueOnError)
	fs.SetOutput(stderr)
	var addTags, removeTags []string
	var name, desc string
	var setName, setDesc bool
	fs.StringArrayVar(&addTags, "add-tag", nil, "tag to add (repeatable)")
	fs.StringArrayVar(&removeTags, "remove-tag", nil, "tag to remove (repeatable)")
	fs.StringVar(&name, "name", "", "new name")
	fs.StringVar(&desc, "description", "", "new description")
	if err := fs.Parse(args[1:]); err != nil {
		return ExitUsage
	}
	fs.Visit(func(f *pflag.Flag) {
		if f.Name == "name" {
			setName = true
		}
		if f.Name == "description" {
			setDesc = true
		}
	})

	sess, err := st.GetSession(ctx, id)
	if err != nil {
		fmt.Fprintf(stderr, "get session: %v\n", err)
		return ExitUsage
	}
	if setName {
		sess.Name = name
	}
	if setDesc {
		sess.Description = desc
	}
	tagSet := make(map[string]struct{}, len(sess.Tags))
	for _, t := range sess.Tags {
		tagSet[t] = struct{}{}
	}
	for _, t := range addTags {
		tagSet[t] = struct{}{}
	}
	for _, t := range removeTags {
		delete(tagSet, t)
	}
	merged := make([]string, 0, len(tagSet))
	for t := range tagSet {
		merged = append(merged, t)
	}
	sort.Strings(merged)
	sess.Tags = merged
	if err := st.SaveSession(ctx, sess); err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return ExitUsage
	}
	fmt.Fprintf(stdout, "updated session %s (tags: %s)\n", sess.ID, strings.Join(sess.Tags, ","))
	return ExitOK
}

// buildMatchers parses each --tag value into a Matcher; the resulting
// list is AND-joined (a session must satisfy every Matcher).
func buildMatchers(exprs []string) ([]tagfilter.Matcher, error) {
	out := make([]tagfilter.Matcher, 0, len(exprs))
	for _, e := range exprs {
		m, err := tagfilter.Parse(e)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func matchAll(ms []tagfilter.Matcher, tags []string) bool {
	for _, m := range ms {
		if !m.Matches(tags) {
			return false
		}
	}
	return true
}
