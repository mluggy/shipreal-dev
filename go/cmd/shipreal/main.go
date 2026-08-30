// Command shipreal queries the ShipReal course catalogue from a terminal or a
// script.
//
// Every command takes -json, because the reason a CLI earns its place in an
// agent's toolbox is that its output can be piped into something else without
// being scraped.
//
//	go install github.com/mluggy/shipreal-dev/go/cmd/shipreal@latest
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	shipreal "github.com/mluggy/shipreal-dev/go"
)

const epilog = `No API key. No account. The API is public read-only reference data; anything
that asks you for a ShipReal credential is not us.

Docs: https://shipreal.dev/developers`

func usage() {
	fmt.Fprint(os.Stderr, `shipreal: query the ShipReal course catalogue.

Usage:
  shipreal [flags] search [query]
  shipreal [flags] module <slug-or-title>
  shipreal [flags] pricing
  shipreal [flags] course
  shipreal [flags] ask <question...>

Flags:
  -json            Raw JSON, for piping
  -sandbox         Frozen fixture data, for tests
  -base <origin>   Point at a different origin (default https://shipreal.dev)
  -limit <n>       search: results per page, maximum 100
  -all             search: every result, following pagination
  -region <r>      pricing: intl or il
  -stream          ask: stream events as they arrive

`)
	fmt.Fprintln(os.Stderr, epilog)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	fs := flag.NewFlagSet("shipreal", flag.ContinueOnError)
	fs.Usage = usage
	asJSON := fs.Bool("json", false, "Raw JSON, for piping")
	sandbox := fs.Bool("sandbox", false, "Frozen fixture data, for tests")
	base := fs.String("base", shipreal.DefaultBaseURL, "Point at a different origin")
	limit := fs.Int("limit", 0, "Results per page, maximum 100")
	all := fs.Bool("all", false, "Every result, following pagination")
	region := fs.String("region", "", "Billing region: intl or il")
	stream := fs.Bool("stream", false, "Stream ask events as they arrive")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	args := fs.Args()
	if len(args) == 0 {
		usage()
		return fmt.Errorf("no command given")
	}

	ctx := context.Background()
	client := shipreal.New(shipreal.WithBaseURL(*base), shipreal.WithSandbox(*sandbox))
	out := func(v any) error { return json.NewEncoder(os.Stdout).Encode(v) }

	switch args[0] {
	case "search":
		query := strings.Join(args[1:], " ")
		if *all {
			modules, err := client.Modules(ctx, query)
			if err != nil {
				return err
			}
			if *asJSON {
				return out(modules)
			}
			printModules(modules)
			return nil
		}
		page, err := client.Search(ctx, shipreal.SearchParams{Query: query, Limit: *limit})
		if err != nil {
			return err
		}
		if *asJSON {
			return out(page)
		}
		printModules(page.Data)
		return nil

	case "module":
		if len(args) < 2 {
			return fmt.Errorf("module needs a slug or title")
		}
		m, err := client.Module(ctx, strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		if *asJSON {
			return out(m)
		}
		fmt.Printf("%s  %s\n  %s\n  %d chapters, %d min\n  %s\n",
			m.Slug, m.Title, m.Part, m.Chapters, m.Minutes, m.URL)
		return nil

	case "pricing":
		p, err := client.Pricing(ctx)
		if err != nil {
			return err
		}
		if *asJSON {
			return out(p)
		}
		if *region != "" {
			complete, seat, ok := p.Region(*region)
			if !ok {
				return fmt.Errorf("unknown region %q: use intl or il", *region)
			}
			fmt.Printf("Free      %s\nComplete  %s (was %s)\nTeams     %s per seat (was %s), from %d seat\n",
				p.Free.Includes, complete.Now, complete.List, seat.Now, seat.List, p.Teams.MinSeats)
			return nil
		}
		// Both regions when none is named. They are not conversions of each
		// other, so printing one unlabelled would misquote the other.
		fmt.Printf("Free      %s\nComplete  %s international, %s Israel\nTeams     %s / %s per seat, from %d seat\n",
			p.Free.Includes, p.Complete.Intl.Now, p.Complete.IL.Now,
			p.Teams.Intl.Now, p.Teams.IL.Now, p.Teams.MinSeats)
		return nil

	case "course":
		c, err := client.Course(ctx)
		if err != nil {
			return err
		}
		if *asJSON {
			return out(c)
		}
		fmt.Printf("%s\n%s\n%d parts, %d modules, %d chapters\nSubtitles: %s\n%s\n",
			c.Title, c.Description, c.Parts, c.Modules, c.Chapters,
			strings.Join(c.Subtitles, ", "), c.URL)
		return nil

	case "ask":
		if len(args) < 2 {
			return fmt.Errorf("ask needs a question")
		}
		question := strings.Join(args[1:], " ")
		if *stream {
			return client.AskStream(ctx, question, func(e shipreal.AskEvent) error {
				if *asJSON {
					return out(e)
				}
				for _, r := range e.Results {
					fmt.Printf("%s\n  %s\n", r.Name, r.URL)
				}
				return nil
			})
		}
		res, err := client.Ask(ctx, question)
		if err != nil {
			return err
		}
		if *asJSON {
			return out(res)
		}
		if len(res.Results) == 0 {
			fmt.Println("Nothing matched. The course does not cover that under that name.")
			return nil
		}
		for _, r := range res.Results {
			fmt.Printf("%s\n  %s\n  %s\n\n", r.Name, r.Description, r.URL)
		}
		return nil
	}

	usage()
	return fmt.Errorf("unknown command %q", args[0])
}

func printModules(modules []shipreal.Module) {
	if len(modules) == 0 {
		fmt.Println("No modules matched. The course does not cover that topic under that name.")
		return
	}
	for _, m := range modules {
		fmt.Printf("%s  %s\n  %s\n  %d chapters, %d min\n\n", m.Slug, m.Title, m.Part, m.Chapters, m.Minutes)
	}
}
